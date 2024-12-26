package csi

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/zncdatadev/operator-go/pkg/constants"
	"github.com/zncdatadev/secret-operator/internal/csi/backend"
	"github.com/zncdatadev/secret-operator/pkg/pod_info"
	"github.com/zncdatadev/secret-operator/pkg/volume"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	KubedoopTOPOLOGY = "secrets.kubedoop.dev/node"
)

var (
	volumeCaps = []*csi.VolumeCapability_AccessMode{
		{
			Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
		},
		{
			Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
		},
		{
			Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY,
		},
	}
)

type ControllerServer struct {
	csi.UnimplementedControllerServer
	client  client.Client
	volumes map[string]int64
}

var _ csi.ControllerServer = &ControllerServer{}

func NewControllerServer(client client.Client) *ControllerServer {
	return &ControllerServer{
		client:  client,
		volumes: map[string]int64{},
	}
}

func (c *ControllerServer) CreateVolume(ctx context.Context, request *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	if err := validateCreateVolumeRequest(request); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	requiredCap := request.CapacityRange.GetRequiredBytes()
	if existCap, ok := c.volumes[request.Name]; ok && existCap < requiredCap {
		return nil, status.Errorf(codes.AlreadyExists, "Volume: %q, capacity bytes: %d", request.Name, requiredCap)
	}

	c.volumes[request.Name] = requiredCap
	if request.Parameters["secretFinalizer"] == "true" {
		logger.V(1).Info("Finalizer is true")
	}

	// requests.parameters is StorageClass.Parameters, which is set by user when creating PVC.
	// When adding '--extra-create-metadata' args in sidecar of registry.k8s.io/sig-storage/csi-provisioner container,
	// we can get the following parameters from requests.parameters:
	// - 'csi.storage.k8s.io/pv/name'
	// - 'csi.storage.k8s.io/pvc/name'
	// - 'csi.storage.k8s.io/pvc/namespace'
	// ref: https://github.com/kubernetes-csi/external-provisioner?tab=readme-ov-file#command-line-options
	pvcObjectKey, err := c.getPVCObjectKey(request.Parameters)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	pvc, err := c.getPVC(ctx, pvcObjectKey)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	volumeContext, err := c.getVolumeContext(pvc)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// get accessible topology
	accessibleTopology, err := c.getAssibleTopology(ctx, pvc)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:           request.GetName(),
			CapacityBytes:      requiredCap,
			VolumeContext:      volumeContext,
			AccessibleTopology: accessibleTopology,
		},
	}, nil
}

func (c *ControllerServer) getAssibleTopology(ctx context.Context, pvc *corev1.PersistentVolumeClaim) ([]*csi.Topology, error) {
	volumeContext, err := volume.NewvolumeContextFromMap(pvc.Annotations)
	if err != nil {
		return nil, err
	}

	pod := &corev1.Pod{}
	if err := c.client.Get(ctx, client.ObjectKey{Namespace: pvc.Namespace, Name: pvc.OwnerReferences[0].Name}, pod); err != nil {
		return nil, err
	}

	podInto := pod_info.NewPodInfo(c.client, pod, &volumeContext.Scope)

	backend, err := backend.NewBackend(ctx, c.client, podInto, volumeContext)

	nodeNames, err := backend.GetQualifiedNodeNames(ctx)
	if err != nil {
		return nil, err
	}

	if len(nodeNames) == 0 {
		return nil, nil
	}

	topology := make([]*csi.Topology, 0, len(nodeNames))
	for _, nodeName := range nodeNames {
		topology = append(topology, &csi.Topology{
			Segments: map[string]string{
				KubedoopTOPOLOGY: nodeName,
			},
		})
	}
	return nil, nil

}

func (c *ControllerServer) getPVC(ctx context.Context, objectKey client.ObjectKey) (*corev1.PersistentVolumeClaim, error) {
	pvc := &corev1.PersistentVolumeClaim{}
	if err := c.client.Get(ctx, objectKey, pvc); err != nil {
		return nil, err
	}
	return pvc, nil
}

func (c *ControllerServer) getPVCObjectKey(requestParams map[string]string) (client.ObjectKey, error) {
	pvcName, pvcNameExists := requestParams["csi.storage.k8s.io/pvc/name"]
	pvcNamespace, pvcNamespaceExists := requestParams["csi.storage.k8s.io/pvc/namespace"]

	if pvcNameExists && pvcNamespaceExists {
		return client.ObjectKey{
			Namespace: pvcNamespace,
			Name:      pvcName,
		}, nil
	}

	return client.ObjectKey{}, fmt.Errorf("ensure '--extra-create-metadata' args are added in the sidecar of the csi-provisioner container.")
}

func (c *ControllerServer) getVolumeContext(pvc *corev1.PersistentVolumeClaim) (map[string]string, error) {
	volumeContext := pvc.Annotations
	if _, ok := volumeContext[constants.AnnotationSecretsClass]; ok {
		return nil, fmt.Errorf("required annotations %s, not found in pvc %s, namespace %s", constants.AnnotationSecretsClass, pvc.Name, pvc.Namespace)
	}
	return volumeContext, nil
}

func (c *ControllerServer) DeleteVolume(ctx context.Context, request *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {

	if err := c.validateDeleteVolumeRequest(request); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// check pv if dynamic
	dynamic, err := CheckDynamicPV(request.GetVolumeId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Check Volume ID error: %v", err)
	}

	if !dynamic {
		logger.V(5).Info("Volume is not dynamic, skip delete volume")
		return &csi.DeleteVolumeResponse{}, nil
	}

	if _, ok := c.volumes[request.VolumeId]; !ok {
		// return nil, status.Errorf(codes.NotFound, "Volume ID: %q", request.VolumeId)
		logger.V(1).Info("Volume not found, skip delete volume")
	}

	return &csi.DeleteVolumeResponse{}, nil
}

func (c *ControllerServer) validateDeleteVolumeRequest(request *csi.DeleteVolumeRequest) error {
	if request.VolumeId == "" {
		return errors.New("volume ID is required")
	}

	return nil
}

func (c *ControllerServer) ControllerPublishVolume(ctx context.Context, request *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (c *ControllerServer) ControllerUnpublishVolume(ctx context.Context, request *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (c *ControllerServer) ValidateVolumeCapabilities(ctx context.Context, request *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {

	if request.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID is required")
	}

	vcs := request.GetVolumeCapabilities()

	if len(vcs) == 0 {
		return nil, status.Error(codes.InvalidArgument, "VolumeCapabilities is required")
	}

	return &csi.ValidateVolumeCapabilitiesResponse{
		Confirmed: &csi.ValidateVolumeCapabilitiesResponse_Confirmed{
			VolumeCapabilities: request.VolumeCapabilities,
		},
	}, nil
}

func (c *ControllerServer) ListVolumes(ctx context.Context, request *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	// impl list volumes
	entries := make([]*csi.ListVolumesResponse_Entry, 0, len(c.volumes))
	for volumeID, size := range c.volumes {
		entries = append(entries, &csi.ListVolumesResponse_Entry{
			Volume: &csi.Volume{
				VolumeId:           volumeID,
				CapacityBytes:      size,
				VolumeContext:      nil,
				ContentSource:      nil,
				AccessibleTopology: nil,
			},
		})
	}

	return &csi.ListVolumesResponse{
		Entries: entries,
	}, nil

}

func (c *ControllerServer) ControllerGetCapabilities(ctx context.Context, request *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {

	return &csi.ControllerGetCapabilitiesResponse{
		Capabilities: []*csi.ControllerServiceCapability{
			{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
					},
				},
			},
		},
	}, nil
}

func isValidVolumeCapabilities(volCaps []*csi.VolumeCapability) bool {
	foundAll := true
	for _, c := range volCaps {
		if !isSupportVolumeCapabilities(c) {
			foundAll = false
		}
	}
	return foundAll
}

// isSupportVolumeCapabilities checks if the volume capabilities are supported by the driver
func isSupportVolumeCapabilities(cap *csi.VolumeCapability) bool {
	switch cap.GetAccessType().(type) {
	case *csi.VolumeCapability_Block:
		return false
	case *csi.VolumeCapability_Mount:
		break
	default:
		return false
	}
	for _, volumeCap := range volumeCaps {
		if volumeCap.GetMode() == cap.AccessMode.GetMode() {
			return true
		}
	}
	return false
}

func CheckDynamicPV(name string) (bool, error) {
	return regexp.Match("pvc-\\w{8}(-\\w{4}){3}-\\w{12}", []byte(name))
}

func validateCreateVolumeRequest(request *csi.CreateVolumeRequest) error {
	if request.GetName() == "" {
		return errors.New("volume Name is required")
	}

	if request.GetCapacityRange() == nil {
		return errors.New("capacityRange is required")
	}

	if request.GetVolumeCapabilities() == nil {
		return errors.New("volumeCapabilities is required")
	}

	if !isValidVolumeCapabilities(request.GetVolumeCapabilities()) {
		return errors.New("volumeCapabilities is not supported")
	}

	return nil
}
