package csi

import (
	"context"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/protobuf/ptypes/wrappers"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ csi.IdentityServer = &IdentityServer{}

type IdentityServer struct {
	name    string
	version string
}

func NewIdentityServer(name, version string) *IdentityServer {
	return &IdentityServer{
		name:    name,
		version: version,
	}

}

func (i IdentityServer) GetPluginInfo(ctx context.Context, request *csi.GetPluginInfoRequest) (*csi.GetPluginInfoResponse, error) {
	if i.name == "" {
		return nil, status.Error(codes.Unavailable, "driver name not configured")
	}

	if i.version == "" {
		return nil, status.Error(codes.Unavailable, "driver version not configured")
	}

	return &csi.GetPluginInfoResponse{
		Name:          i.name,
		VendorVersion: i.version,
	}, nil
}

func (i IdentityServer) GetPluginCapabilities(ctx context.Context, request *csi.GetPluginCapabilitiesRequest) (*csi.GetPluginCapabilitiesResponse, error) {

	return &csi.GetPluginCapabilitiesResponse{
		Capabilities: []*csi.PluginCapability{
			{
				Type: &csi.PluginCapability_Service_{
					Service: &csi.PluginCapability_Service{
						Type: csi.PluginCapability_Service_CONTROLLER_SERVICE,
					},
				},
			},
		},
	}, nil
}

func (i IdentityServer) Probe(ctx context.Context, request *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	return &csi.ProbeResponse{Ready: &wrappers.BoolValue{Value: true}}, nil
}
