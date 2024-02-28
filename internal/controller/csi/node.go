package csi

import (
	"context"
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
	//volumeId := request.GetVolumeId()
	//targetPath := request.GetTargetPath()
	//stagingTargetPath := request.GetStagingTargetPath()
	return nil, status.Error(codes.Unimplemented, "")
}

func (n NodeServer) NodeUnpublishVolume(ctx context.Context, request *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
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
	newCapabilities := func(cap csi.NodeServiceCapability_RPC_Type) *csi.NodeServiceCapability {
		return &csi.NodeServiceCapability{
			Type: &csi.NodeServiceCapability_Rpc{
				Rpc: &csi.NodeServiceCapability_RPC{
					Type: cap,
				},
			},
		}
	}

	var capabilities []*csi.NodeServiceCapability

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

func (n NodeServer) NodeGetInfo(ctx context.Context, request *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	log.V(5).Info("Using default NodeGetInfo")
	return &csi.NodeGetInfoResponse{
		NodeId: n.nodeID,
	}, nil
}
