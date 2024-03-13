package csi

import (
	"context"
	"errors"
	"regexp"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	SECRET_CLASS_ANNOTATION_NAME = "secrets.zncdata.dev/class"
	SECRET_SCOPE_ANNOTATION_NAME = "secrets.zncdata.dev/scope"
)

var (
	volumeCaps = []csi.VolumeCapability_AccessMode{
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
	client  client.Client
	volumes map[string]int64
}

// SecretClassInfo is the struct for secret class info in PVC annotations
type SecretClassInfo struct {
	Name  string `json:"secretClassName"`
	Scope string `json:"scope"`
}

var _ csi.ControllerServer = &ControllerServer{}

func NewControllerServer(client client.Client) *ControllerServer {
	return &ControllerServer{
		client:  client,
		volumes: map[string]int64{},
	}
}

func (c ControllerServer) CreateVolume(ctx context.Context, request *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	if request.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume Name is required")
	}

	if request.GetCapacityRange() == nil {
		return nil, status.Error(codes.InvalidArgument, "CapacityRange is required")
	}

	if request.GetVolumeCapabilities() == nil {
		return nil, status.Error(codes.InvalidArgument, "VolumeCapabilities is required")
	}

	if !isValidVolumeCapabilities(request.GetVolumeCapabilities()) {
		return nil, status.Error(codes.InvalidArgument, "VolumeCapabilities is not supported")
	}

	requiredCap := request.CapacityRange.GetRequiredBytes()
	if existCap, ok := c.volumes[request.Name]; ok && existCap < requiredCap {
		return nil, status.Errorf(codes.AlreadyExists, "Volume: %q, capacity bytes: %d", request.Name, requiredCap)
	}

	if request.Parameters["secretFinalizer"] == "true" {
		log.Info("Finalizer is true")
	}
	// ref: https://github.com/kubernetes-csi/external-provisioner?tab=readme-ov-file#command-line-options
	pvcName, exists := request.Parameters["csi.storage.k8s.io/pvc/name"]
	if !exists {
		return nil, status.Error(codes.InvalidArgument, "Can not found 'csi.storage.k8s.io/pvc/name' in parameters, "+
			"please ensure added '--extra-create-metadata' args in sidecar of registry.k8s.io/sig-storage/csi-provisioner container.")
	}

	pvcNameSpace, exists := request.Parameters["csi.storage.k8s.io/pvc/namespace"]
	if !exists {
		return nil, status.Error(codes.InvalidArgument, "Can not found 'csi.storage.k8s.io/pvc/namespace' in parameters, "+
			"please ensure added '--extra-create-metadata' args in sidecar of registry.k8s.io/sig-storage/csi-provisioner container.")
	}

	pvc, err := c.getPvc(pvcName, pvcNameSpace)

	if err != nil {
		return nil, status.Errorf(codes.NotFound, "PVC: %q, Namespace: %q", pvcName, pvcNameSpace)
	}

	secretClassInfo, err := c.getSecretClassInfo(pvc)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Get secret class info error: %v", err)
	}

	volCtx := map[string]string{}

	if secretClassInfo != nil {
		volCtx["secretClassName"] = secretClassInfo.Name
		volCtx["secretScope"] = secretClassInfo.Scope
	}

	c.volumes[request.Name] = requiredCap

	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      request.GetName(),
			CapacityBytes: requiredCap,
			VolumeContext: volCtx,
		},
	}, nil
}

func (c ControllerServer) getPvc(name, namespace string) (*corev1.PersistentVolumeClaim, error) {

	pvc := &corev1.PersistentVolumeClaim{}
	err := c.client.Get(context.Background(), client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, pvc)
	if err != nil {
		return nil, err
	}

	return pvc, nil
}

func (c ControllerServer) getSecretClassInfo(pvc *corev1.PersistentVolumeClaim) (*SecretClassInfo, error) {

	annotations := pvc.GetAnnotations()
	if annotations == nil {
		return nil, errors.New("PVC annotations is nil")
	}

	secretClassName, ok := annotations[SECRET_SCOPE_ANNOTATION_NAME]
	if !ok {
		return nil, errors.New("can not found '" + SECRET_CLASS_ANNOTATION_NAME + "' annotation in PVC")
	}

	secretScope, ok := annotations[SECRET_SCOPE_ANNOTATION_NAME]
	if !ok {
		return nil, errors.New("can not found '" + SECRET_SCOPE_ANNOTATION_NAME + "' annotation in PVC")
	}

	return &SecretClassInfo{
		Name:  secretClassName,
		Scope: secretScope,
	}, nil
}

func (c ControllerServer) DeleteVolume(ctx context.Context, request *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	volumeID := request.GetVolumeId()
	if volumeID == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID is required")
	}

	// check pv if dynamic
	dynamic, err := CheckDynamicPV(volumeID)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Check Volume ID error: %v", err)
	}

	if !dynamic {
		log.V(5).Info("Volume is not dynamic, skip delete volume")
		return &csi.DeleteVolumeResponse{}, nil
	}

	if _, ok := c.volumes[request.VolumeId]; !ok {
		return nil, status.Errorf(codes.NotFound, "Volume ID: %q", request.VolumeId)
	}

	return &csi.DeleteVolumeResponse{}, nil
}

func (c ControllerServer) ControllerPublishVolume(ctx context.Context, request *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (c ControllerServer) ControllerUnpublishVolume(ctx context.Context, request *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (c ControllerServer) ValidateVolumeCapabilities(ctx context.Context, request *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {

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

func (c ControllerServer) ListVolumes(ctx context.Context, request *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	// impl list volumes
	var entries []*csi.ListVolumesResponse_Entry
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

func (c ControllerServer) GetCapacity(ctx context.Context, request *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (c ControllerServer) ControllerGetCapabilities(ctx context.Context, request *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {

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

func (c ControllerServer) CreateSnapshot(ctx context.Context, request *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (c ControllerServer) DeleteSnapshot(ctx context.Context, request *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (c ControllerServer) ListSnapshots(ctx context.Context, request *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (c ControllerServer) ControllerExpandVolume(ctx context.Context, request *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (c ControllerServer) ControllerGetVolume(ctx context.Context, request *csi.ControllerGetVolumeRequest) (*csi.ControllerGetVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (c ControllerServer) ControllerModifyVolume(ctx context.Context, request *csi.ControllerModifyVolumeRequest) (*csi.ControllerModifyVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
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
	for _, c := range volumeCaps {
		if c.GetMode() == cap.AccessMode.GetMode() {
			return true
		}
	}
	return false
}

func CheckDynamicPV(name string) (bool, error) {
	return regexp.Match("pvc-\\w{8}(-\\w{4}){3}-\\w{12}", []byte(name))
}
