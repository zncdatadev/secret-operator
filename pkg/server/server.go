package server

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"

	ctrl "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/go-logr/logr"
	"google.golang.org/grpc"

	"github.com/zncdatadev/secret-operator/pkg/util"
)

// NonBlockingServer Defines Non blocking GRPC server interfaces
type NonBlockingServer interface {
	// Start services at the endpoint in non-blocking manner
	Start(ctx context.Context) error
	// Stop Stops the service gracefully
	Stop()
	// ForceStop Stops the service forcefully
	ForceStop()
	// Wait waits for the server to finish
	Wait() error

	// RegisterService registers a service with the gRPC server
	RegisterServiceFunc(registerFunc func(server *grpc.Server))

	// RegisterService registers a service with the gRPC server
	RegisterService(nodeServer csi.NodeServer, identityServer csi.IdentityServer, controllerServer csi.ControllerServer)
}

var _ NonBlockingServer = &server{}

// Server represents a configuration for a gRPC server
type server struct {
	endpoint string
	opts     []grpc.ServerOption

	listener net.Listener
	server   *grpc.Server
	logger   logr.Logger

	// wg is used to wait for the server goroutine to finish
	wg sync.WaitGroup
	// once ensures the listener is closed only once
	// This is used to prevent closing the listener multiple times
	// which can lead to errors if the server is stopped and started again.
	once sync.Once
}

// ServerOption defines a function for configuring the Server
type ServerOption func(*server)

// WithServerOptions allows setting custom gRPC server options
func WithServerOptions(opts ...grpc.ServerOption) ServerOption {
	return func(s *server) {
		s.opts = append(s.opts, opts...)
	}
}

// WithLogger allows setting a custom logger
func WithLogger(logger logr.Logger) ServerOption {
	return func(s *server) {
		s.logger = logger
	}
}

func NewNonBlockingServer(endpoint string, opts ...ServerOption) NonBlockingServer {
	defaultOpts := []grpc.ServerOption{
		grpc.UnaryInterceptor(util.LogGRPC),
	}

	s := &server{
		endpoint: endpoint,
		opts:     defaultOpts,
		logger:   ctrl.Log.WithName("csi-server"),
	}

	for _, opt := range opts {
		opt(s)
	}

	s.server = grpc.NewServer(s.opts...)

	return s
}

func (s *server) Start(ctx context.Context) error {
	listener, err := s.createListener()
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}
	s.listener = listener

	// Start the gRPC server in a goroutine
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.server.Serve(s.listener); err != nil {
			s.logger.Error(err, "failed to serve gRPC", "endpoint", s.endpoint)
		}
	}()

	return nil
}

func (s *server) Stop() {
	s.logger.V(1).Info("stopping server gracefully", "endpoint", s.endpoint)

	if s.server != nil {
		s.server.GracefulStop()
		s.wg.Wait()       // Wait for the server goroutine to finish
		s.closeListener() // Close the listener safely
		s.logger.V(1).Info("server stopped gracefully", "endpoint", s.endpoint)
	}
}

func (s *server) ForceStop() {
	s.logger.V(1).Info("stopping server forcefully", "endpoint", s.endpoint)

	if s.server != nil {
		s.server.Stop()
		s.closeListener() // Close the listener safely
		s.logger.V(1).Info("server stopped forcefully", "endpoint", s.endpoint)
	}
}

// Wait blocks until the server is stopped and returns any error that caused the stop
func (s *server) Wait() error {
	s.logger.V(1).Info("waiting for server to finish", "endpoint", s.endpoint)

	// Wait for the server goroutine to finish
	s.wg.Wait()

	// Close the listener safely
	s.closeListener()

	s.logger.V(1).Info("server has finished", "endpoint", s.endpoint)
	return nil
}

func (s *server) RegisterServiceFunc(registerFunc func(server *grpc.Server)) {
	if s.server == nil {
		s.logger.Error(fmt.Errorf("server is not initialized"), "failed to register service")
		return
	}
	registerFunc(s.server)
	s.logger.V(1).Info("service registered", "endpoint", s.endpoint)
}

func (s *server) RegisterService(nodeServer csi.NodeServer, identityServer csi.IdentityServer, controllerServer csi.ControllerServer) {
	if s.server == nil {
		s.logger.Error(fmt.Errorf("server is not initialized"), "failed to register service")
		return
	}

	if nodeServer != nil {
		csi.RegisterNodeServer(s.server, nodeServer)
	}
	if identityServer != nil {
		csi.RegisterIdentityServer(s.server, identityServer)
	}
	if controllerServer != nil {
		csi.RegisterControllerServer(s.server, controllerServer)
	}

	s.logger.V(1).Info("csi services registered", "endpoint", s.endpoint)
}

// createListener creates a network listener based on the endpoint
func (s *server) createListener() (net.Listener, error) {
	proto, addr, err := util.ParseEndpoint(s.endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to parse endpoint: %w", err)
	}

	if proto == "unix" {
		addr = "/" + addr
		if err := s.removeUnixSocket(addr); err != nil {
			return nil, fmt.Errorf("failed to remove existing unix socket: %w", err)
		}
	}

	listener, err := net.Listen(proto, addr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s://%s: %w", proto, addr, err)
	}

	return listener, nil
}

// removeUnixSocket removes existing unix socket file if it exists
func (s *server) removeUnixSocket(addr string) error {
	if err := os.Remove(addr); err != nil && !os.IsNotExist(err) {
		s.logger.V(0).Info("Failed to remove existing unix socket", "addr", addr, "error", err.Error())
		return err
	}
	return nil
}

// closeListener safely closes the listener only once
func (s *server) closeListener() {
	s.once.Do(func() {
		if s.listener != nil {
			if err := s.listener.Close(); err != nil {
				s.logger.Error(err, "failed to close listener", "endpoint", s.endpoint)
			}
		}
	})
}
