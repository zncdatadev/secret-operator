package csi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"time"

	"io/fs"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/mount"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/zncdatadev/operator-go/pkg/constants"
	secretsv1alpha1 "github.com/zncdatadev/secret-operator/api/v1alpha1"
	secretbackend "github.com/zncdatadev/secret-operator/internal/csi/backend"

	"github.com/zncdatadev/secret-operator/pkg/pod_info"
	"github.com/zncdatadev/secret-operator/pkg/volume"
)

var _ csi.NodeServer = &NodeServer{}

type NodeServer struct {
	csi.UnimplementedNodeServer
	mounter mount.Interface
	nodeID  string
	client  client.Client
}

func NewNodeServer(
	nodeId string,
	mounter mount.Interface,
	client client.Client,
) *NodeServer {
	return &NodeServer{
		nodeID:  nodeId,
		mounter: mounter,
		client:  client,
	}
}

func (n *NodeServer) NodePublishVolume(ctx context.Context, request *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	// check requests
	// 	- volume ID missing in request
	// 	- target path missing in request
	// 	- volume capability missing in request
	// 	- volume context missing in request or empty
	if err := n.validateNodePublishVolumeRequest(request); err != nil {
		return nil, err
	}

	targetPath := request.GetTargetPath()
	volumeID := request.GetVolumeId()

	// get the volume context
	// Default, volume context contains data:
	//   - csi.storage.k8s.io/pod.name: <pod-name>
	//   - csi.storage.k8s.io/pod.namespace: <pod-namespace>
	//   - csi.storage.k8s.io/pod.uid: <pod-uid>
	//   - csi.storage.k8s.io/serviceAccount.name: <service-account-name>
	//   - csi.storage.k8s.io/ephemeral: <true|false>
	//   - storage.kubernetes.io/csiProvisionerIdentity: <provisioner-identity>
	//   - volume.kubernetes.io/storage-provisioner: <provisioner-name>
	//   - volume.beta.kubernetes.io/storage-provisioner: <provisioner-name>
	// If you need more information about PVC, you should pass it to CreateVolumeResponse.Volume.VolumeContext
	// when called CreateVolume response in the controller side. Then use them here.
	// In this csi, we can get PVC annotations from volume context,
	// because we deliver it from controller to node already.
	// The following PVC annotations is required:
	//   - secrets.zncdata.dev/class: <secret-class-name>
	volumeSelector, err := volume.NewVolumeSelectorFromMap(request.GetVolumeContext())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if volumeSelector.Class == "" {
		return nil, status.Error(codes.InvalidArgument, "Secret class name missing in request")
	}

	secretClass := &secretsv1alpha1.SecretClass{}
	// get the secret class
	// SecretClass is cluster coped, so we don't need to specify the namespace
	if err := n.client.Get(ctx, client.ObjectKey{
		Name: volumeSelector.Class,
	}, secretClass); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// get the pod
	pod := &corev1.Pod{}
	if err := n.client.Get(ctx, client.ObjectKey{
		Name:      volumeSelector.Pod,
		Namespace: volumeSelector.PodNamespace,
	}, pod); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	podInfo := pod_info.NewPodInfo(n.client, pod, volumeSelector)

	// get the secret data
	backend := secretbackend.NewBackend(n.client, podInfo, volumeSelector, secretClass)
	secretContent, err := backend.GetSecretData(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// mount the volume to the target path
	if err := n.mount(targetPath); err != nil {
		return nil, err
	}

	// write the secret data to the target path
	if err := n.writeData(targetPath, secretContent.Data); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// update the pod annotation with the secret expiration time
	if err := n.updatePod(ctx, pod.DeepCopy(), volumeID, secretContent.ExpiresTime); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &csi.NodePublishVolumeResponse{}, nil
}

// updatePod updates the pod annotation with the secret expiration time.
// The volume ID is hashed using sha256, and the first 16 bytes are used as the volume tag.
// Then, the expiration time is written to the pod annotation with the key "secrets.zncdata.dev/restarter-expires-at:<volume_tag>".
//
// Considering the length 63 limitation of Kubernetes annotations, we hash the volume ID to maintain the readability of the annotation
// and its association with the volume. However, truncating the hash to the first 16 bytes may introduce collision risks.
func (n *NodeServer) updatePod(ctx context.Context, pod *corev1.Pod, volumeID string, expiresTime *time.Time) error {
	if pod.Annotations == nil {
		pod.Annotations = make(map[string]string)
	}
	patch := client.MergeFrom(pod.DeepCopy())
	if expiresTime == nil {
		logger.V(5).Info("Expiration time is nil, skip update pod annotation", "pod", pod.Name)
		return nil
	}

	volumeTagHash := sha256.New()
	volumeTagHash.Write([]byte("secrets.zncdata.dev/volume:"))
	volumeTagHash.Write([]byte(volumeID))
	volumeTag := volumeTagHash.Sum(nil)
	// get 16 bytes of volume tag, but it maybe cause collision vulnerability
	volumeTag = volumeTag[:16]

	annotationExpiresName := constants.PrefixLabelRestarterExpiresAt + hex.EncodeToString(volumeTag)
	expiresTimeStr := expiresTime.Format(time.RFC3339)
	logger.V(5).Info("Update pod annotation", "pod", pod.Name, "key", annotationExpiresName, "value", expiresTimeStr)

	pod.Annotations[annotationExpiresName] = expiresTimeStr

	if err := n.client.Patch(ctx, pod, patch); err != nil {
		return err
	}
	logger.V(5).Info("Pod patched", "pod", pod.Name)
	return nil
}

// writeData writes the data to the target path.
// The data is a map of key-value pairs.
// The key is the file name, and the value is the file content.
func (n *NodeServer) writeData(targetPath string, data map[string]string) error {
	for name, content := range data {
		fileName := filepath.Join(targetPath, name)
		if err := os.WriteFile(fileName, []byte(content), fs.FileMode(0644)); err != nil {
			return err
		}
		logger.V(5).Info("File written", "file", fileName)
	}
	logger.V(5).Info("Data written", "target", targetPath)
	return nil
}

// mount mounts the volume to the target path.
// Mount the volume to the target path with tmpfs.
// The target path is created if it does not exist.
// The volume is mounted with the following options:
//   - noexec (no execution)
//   - nosuid (no set user ID)
//   - nodev (no device)
func (n *NodeServer) mount(targetPath string) error {
	// check if the target path exists
	// if not, create the target path
	// if exists, return error
	if exist, err := mount.PathExists(targetPath); err != nil {
		logger.Error(err, "failed to check if target path exists", "target", targetPath)
		return status.Error(codes.Internal, err.Error())
	} else if exist {
		err := errors.New("target path already exists")
		logger.Error(err, "failed to create target path", "target", targetPath)
		return status.Error(codes.Internal, err.Error())
	} else {
		if err := os.MkdirAll(targetPath, 0750); err != nil {
			logger.Error(err, "failed to create target path", "target", targetPath)
			return status.Error(codes.Internal, err.Error())
		}
	}

	opts := []string{
		"noexec",
		"nosuid",
		"nodev",
	}

	// mount the volume to the target path
	if err := n.mounter.Mount("tmpfs", targetPath, "tmpfs", opts); err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	logger.V(1).Info("Volume mounted", "source", "tmpfs", "target", targetPath, "fsType", "tmpfs", "options", opts)
	return nil
}

// NodeUnpublishVolume unpublishes the volume from the node.
// unmount the volume from the target path, and remove the target path
func (n *NodeServer) NodeUnpublishVolume(ctx context.Context, request *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	// check requests
	if request.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	if request.GetTargetPath() == "" {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}

	targetPath := request.GetTargetPath()

	// unmount the volume from the target path
	if err := n.mounter.Unmount(targetPath); err != nil {
		// FIXME: use status.Error to return error
		// return nil, status.Error(codes.Internal, err.Error())
		logger.V(0).Info("Volume not found, skip delete volume")
	}

	// remove the target path
	if err := os.RemoveAll(targetPath); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (n *NodeServer) validateNodePublishVolumeRequest(request *csi.NodePublishVolumeRequest) error {
	if request.GetVolumeId() == "" {
		return status.Error(codes.InvalidArgument, "volume ID missing in request")
	}
	if request.GetTargetPath() == "" {
		return status.Error(codes.InvalidArgument, "Target path missing in request")
	}
	if request.GetVolumeCapability() == nil {
		return status.Error(codes.InvalidArgument, "Volume capability missing in request")
	}

	if request.GetVolumeContext() == nil || len(request.GetVolumeContext()) == 0 {
		return status.Error(codes.InvalidArgument, "Volume context missing in request")
	}
	return nil
}

func (n *NodeServer) NodeStageVolume(ctx context.Context, request *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	if len(request.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}

	if len(request.GetStagingTargetPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Staging target path missing in request")

	}

	if request.GetVolumeCapability() == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume capability missing in request")
	}

	return &csi.NodeStageVolumeResponse{}, nil
}

func (n *NodeServer) NodeUnstageVolume(ctx context.Context, request *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {

	if len(request.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}

	if len(request.GetStagingTargetPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Staging target path missing in request")

	}

	return &csi.NodeUnstageVolumeResponse{}, nil
}

func (n *NodeServer) NodeGetCapabilities(ctx context.Context, request *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	newCapabilities := func(cap csi.NodeServiceCapability_RPC_Type) *csi.NodeServiceCapability {
		return &csi.NodeServiceCapability{
			Type: &csi.NodeServiceCapability_Rpc{
				Rpc: &csi.NodeServiceCapability_RPC{
					Type: cap,
				},
			},
		}
	}

	capabilities := make([]*csi.NodeServiceCapability, 0, len([]csi.NodeServiceCapability_RPC_Type{
		csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
	}))

	for _, capability := range []csi.NodeServiceCapability_RPC_Type{
		csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
	} {
		capabilities = append(capabilities, newCapabilities(capability))
	}

	resp := &csi.NodeGetCapabilitiesResponse{
		Capabilities: capabilities,
	}

	return resp, nil

}

func (n *NodeServer) NodeGetInfo(ctx context.Context, request *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	return &csi.NodeGetInfoResponse{
		NodeId: n.nodeID,
	}, nil
}
