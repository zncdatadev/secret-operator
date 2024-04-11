package csi

import (
	"net"
	"os"
	"sync"
	"time"

	"google.golang.org/grpc/reflection"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc"

	"github.com/zncdata-labs/secret-operator/pkg/util"
)

// NonBlockingServer Defines Non blocking GRPC server interfaces
type NonBlockingServer interface {
	// Start services at the endpoint
	Start(endpoint string, ids csi.IdentityServer, cs csi.ControllerServer, ns csi.NodeServer, testMode bool)
	// Wait Waits for the service to stop
	Wait()
	// Stop Stops the service gracefully
	Stop()
	// ForceStop Stops the service forcefully
	ForceStop()
}

func NewNonBlockingServer() NonBlockingServer {
	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(util.LogGRPC),
	}

	server := grpc.NewServer(opts...)
	return &nonBlockingServer{
		grpcSrv: server,
	}
}

// NonBlocking server
type nonBlockingServer struct {
	wg      sync.WaitGroup
	grpcSrv *grpc.Server
}

func (s *nonBlockingServer) Start(endpoint string, ids csi.IdentityServer, cs csi.ControllerServer, ns csi.NodeServer, testMode bool) {

	s.wg.Add(1)

	go s.serveGrpc(endpoint, ids, cs, ns, testMode)
}

func (s *nonBlockingServer) Wait() {
	s.wg.Wait()
}

func (s *nonBlockingServer) Stop() {
	s.grpcSrv.GracefulStop()
}

func (s *nonBlockingServer) ForceStop() {
	s.grpcSrv.Stop()
}

func (s *nonBlockingServer) serveGrpc(endpoint string, ids csi.IdentityServer, cs csi.ControllerServer, ns csi.NodeServer, testMode bool) {

	proto, addr, err := util.ParseEndpoint(endpoint)
	if err != nil {
		log.Error(err, "Failed to parse endpoint")
	}

	if proto == "unix" {
		addr = "/" + addr
		if err := os.Remove(addr); err != nil && !os.IsNotExist(err) {
			log.V(1).Info("Failed to remove", "addr", addr, "error", err.Error())
		}
	}

	listener, err := net.Listen(proto, addr)
	if err != nil {
		log.Error(err, "Failed to listen")
	}

	if ids != nil {
		csi.RegisterIdentityServer(s.grpcSrv, ids)
	}
	if cs != nil {
		csi.RegisterControllerServer(s.grpcSrv, cs)
	}
	if ns != nil {
		csi.RegisterNodeServer(s.grpcSrv, ns)
	}

	// Used to stop the server while running tests
	if testMode {

		s.wg.Done()
		go func() {
			// make sure Serve() is called
			s.wg.Wait()
			time.Sleep(time.Millisecond * 1000)
			s.grpcSrv.GracefulStop()
		}()
	}

	log.Info("Listening for connections on address", "address", listener.Addr())

	reflection.Register(s.grpcSrv)
	err = s.grpcSrv.Serve(listener)
	if err != nil {
		log.Error(err, "Failed to serve grpc server")
	}
}
