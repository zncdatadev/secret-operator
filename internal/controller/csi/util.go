package csi

import (
	"context"
	"fmt"
	"github.com/kubernetes-csi/csi-lib-utils/protosanitizer"
	"google.golang.org/grpc"
	"regexp"
	"strings"
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

func getLogLevel(method string) int {
	if method == "/csi.v1.Identity/Probe" ||
		method == "/csi.v1.Node/NodeGetCapabilities" ||
		method == "/csi.v1.Node/NodeGetVolumeStats" {
		return 8
	}
	return 2
}

func logGRPC(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	level := getLogLevel(info.FullMethod)
	log.V(level).Info("GRPC call: %s", info.FullMethod)
	log.V(level).Info("GRPC request: %s", protosanitizer.StripSecrets(req))

	resp, err := handler(ctx, req)
	if err != nil {
		log.Error(err, "GRPC error")
	} else {
		//klog.V(level).Infof("GRPC response: %s", protosanitizer.StripSecrets(resp))
		log.V(level).Info("GRPC response: %s", protosanitizer.StripSecrets(resp))
	}
	return resp, err
}

func CheckDynamicPV(name string) (bool, error) {
	return regexp.Match("pvc-\\w{8}(-\\w{4}){3}-\\w{12}", []byte(name))
}
