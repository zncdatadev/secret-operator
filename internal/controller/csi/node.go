package csi

import (
	"context"
	"os"
	"path/filepath"

	"io/fs"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/utils/mount"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ csi.NodeServer = &NodeServer{}

type NodeServer struct {
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

func (n NodeServer) NodePublishVolume(ctx context.Context, request *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	//startTime := time.Now()

	var targetPath string

	if request.GetVolumeCapability() == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume capability missing in request")
	}
	if request.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	if request.GetTargetPath() == "" {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}
	if request.GetVolumeContext() == nil || len(request.GetVolumeContext()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume context missing in request")
	}
	//
	targetPath = request.GetTargetPath()
	attrib := request.GetVolumeContext()
	//parameters = request.GetVolumeContext()
	mountFlags := request.GetVolumeCapability().GetMount().GetMountFlags()
	//secrets := request.GetSecrets()

	// create the target path if it doesn't exist
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		if err := os.MkdirAll(targetPath, 0750); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	if err := n.mounter.Mount(attrib["source"], targetPath, "", mountFlags); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	if err := os.WriteFile(filepath.Join(targetPath, "hello.txt"), []byte("Hello, world!"), fs.FileMode(0644)); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	log.Info("NodePublishVolume called...")

	return &csi.NodePublishVolumeResponse{}, nil

}

// NodeUnpublishVolume unpublishes the volume from the node.
// unmount the volume from the target path, and remove the target path
func (n NodeServer) NodeUnpublishVolume(ctx context.Context, request *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {

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
		return nil, status.Error(codes.Internal, err.Error())
	}

	// remove the target path
	if err := os.RemoveAll(targetPath); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	log.Info("NodeUnpublishVolume called...")

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (n NodeServer) NodeStageVolume(ctx context.Context, request *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (n NodeServer) NodeUnstageVolume(ctx context.Context, request *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (n NodeServer) NodeGetVolumeStats(ctx context.Context, request *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (n NodeServer) NodeExpandVolume(ctx context.Context, request *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (n NodeServer) NodeGetCapabilities(ctx context.Context, request *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	// newCapabilities := func(cap csi.NodeServiceCapability_RPC_Type) *csi.NodeServiceCapability {
	// 	return &csi.NodeServiceCapability{
	// 		Type: &csi.NodeServiceCapability_Rpc{
	// 			Rpc: &csi.NodeServiceCapability_RPC{
	// 				Type: cap,
	// 			},
	// 		},
	// 	}
	// }

	// var capabilities []*csi.NodeServiceCapability

	// for _, capability := range []csi.NodeServiceCapability_RPC_Type{
	// 	csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
	// } {
	// 	capabilities = append(capabilities, newCapabilities(capability))
	// }

	// resp := &csi.NodeGetCapabilitiesResponse{
	// 	Capabilities: capabilities,
	// }

	log.Info("NodeGetCapabilities called...")

	return &csi.NodeGetCapabilitiesResponse{}, nil
}

func (n NodeServer) NodeGetInfo(ctx context.Context, request *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	log.V(5).Info("Using default NodeGetInfo")
	return &csi.NodeGetInfoResponse{
		NodeId: n.nodeID,
	}, nil
}
