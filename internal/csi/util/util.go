package util

import (
	"context"
	"fmt"
	"strings"

	"github.com/kubernetes-csi/csi-lib-utils/protosanitizer"
	"google.golang.org/grpc"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	log = ctrl.Log.WithName("csi-util")
)

func ParseEndpoint(ep string) (string, string, error) {
	if strings.HasPrefix(strings.ToLower(ep), "unix://") || strings.HasPrefix(strings.ToLower(ep), "tcp://") {
		s := strings.SplitN(ep, "://", 2)
		if s[1] != "" {
			return s[0], s[1], nil
		}
	}
	return "", "", fmt.Errorf("invalid endpoint: %v", ep)
}

func GetLogLevel(method string) int {
	if method == "/csi.v1.Identity/Probe" ||
		method == "/csi.v1.Node/NodeGetCapabilities" ||
		method == "/csi.v1.Node/NodeGetVolumeStats" {
		return 8
	}
	return 2
}

func LogGRPC(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	level := GetLogLevel(info.FullMethod)
	log.V(level).Info("GRPC calling", "method", info.FullMethod, "request", protosanitizer.StripSecrets(req))

	resp, err := handler(ctx, req)
	if err != nil {
		log.Error(err, "GRPC called error", "method", info.FullMethod)
	} else {
		log.V(level).Info("GRPC called", "method", info.FullMethod, "response", protosanitizer.StripSecrets(resp))
	}
	return resp, err
}
