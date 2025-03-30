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
	client client.Client
}

var _ csi.ControllerServer = &ControllerServer{}

func NewControllerServer(client client.Client) *ControllerServer {
	return &ControllerServer{client: client}
}

func (c *ControllerServer) CreateVolume(ctx context.Context, request *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	if err := validateCreateVolumeRequest(request); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	requiredCap := request.CapacityRange.GetRequiredBytes()

	// requests.parameters is StorageClass.Parameters, which is set by user when creating PVC.
	// When adding '--extra-create-metadata' args in sidecar of registry.k8s.io/sig-storage/csi-provisioner container,
	// we can get the following parameters from requests.parameters:
	// - 'csi.storage.k8s.io/pv/name'
	// - 'csi.storage.k8s.io/pvc/name'
	// - 'csi.storage.k8s.io/pvc/namespace'
	// ref: https://github.com/kubernetes-csi/external-provisioner?tab=readme-ov-file#command-line-options
	params, err := newCreateVolumeRequestParamsFromMap(request.Parameters)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	pvc, err := c.getPVC(ctx, client.ObjectKey{Namespace: params.pvcNamespace, Name: params.PVCName})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	volumeContext, err := c.getVolumeContext(pvc)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// get accessible topology
	accessibleTopology, err := c.getAssibleTopology(ctx, pvc, volumeContext)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:           request.GetName(),
			CapacityBytes:      requiredCap,
			VolumeContext:      volumeContext.ToMap(),
			AccessibleTopology: accessibleTopology,
		},
	}, nil
}

func (c *ControllerServer) getAssibleTopology(
	ctx context.Context,
	pvc *corev1.PersistentVolumeClaim,
	volumeContext *volume.SecretVolumeContext,
) ([]*csi.Topology, error) {
	pod := &corev1.Pod{}
	if err := c.client.Get(ctx, client.ObjectKey{Namespace: pvc.Namespace, Name: pvc.OwnerReferences[0].Name}, pod); err != nil {
		return nil, err
	}

	podInto := pod_info.NewPodInfo(c.client, pod, &volumeContext.Scope)

	backend, err := backend.NewBackend(ctx, c.client, podInto, volumeContext)
	if err != nil {
		return nil, err
	}

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
	return topology, nil
}

func (c *ControllerServer) getPVC(ctx context.Context, objectKey client.ObjectKey) (*corev1.PersistentVolumeClaim, error) {
	pvc := &corev1.PersistentVolumeClaim{}
	if err := c.client.Get(ctx, objectKey, pvc); err != nil {
		return nil, err
	}
	return pvc, nil
}

func (c *ControllerServer) getVolumeContext(pvc *corev1.PersistentVolumeClaim) (*volume.SecretVolumeContext, error) {
	annotation := pvc.GetAnnotations()
	volumeContext, err := volume.NewvolumeContextFromMap(annotation)
	if err != nil {
		return nil, err
	}

	if _, ok := annotation[constants.AnnotationSecretsClass]; !ok {
		return nil, fmt.Errorf("required annotation %s is missing", constants.AnnotationSecretsClass)
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
		logger.V(1).Info("volume is not dynamic, skip delete volume")
		return &csi.DeleteVolumeResponse{}, nil
	}

	return &csi.DeleteVolumeResponse{}, nil
}

func (c *ControllerServer) validateDeleteVolumeRequest(request *csi.DeleteVolumeRequest) error {
	if request.VolumeId == "" {
		return fmt.Errorf("volume ID is required")
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
	return nil, status.Error(codes.Unimplemented, "")
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

type createVolumeRequestParams struct {
	PVCName      string
	pvcNamespace string
}

func newCreateVolumeRequestParamsFromMap(params map[string]string) (*createVolumeRequestParams, error) {
	pvcName, pvcNameExists := params[volume.CSIStoragePVCName]
	pvcNamespace, pvcNamespaceExists := params[volume.CSIStoragePVCNamespace]

	if !pvcNameExists || !pvcNamespaceExists {
		return nil, status.Error(codes.InvalidArgument, "ensure '--extra-create-metadata' args are added in the sidecar of the csi-provisioner container.")
	}

	return &createVolumeRequestParams{
		PVCName:      pvcName,
		pvcNamespace: pvcNamespace,
	}, nil
}
