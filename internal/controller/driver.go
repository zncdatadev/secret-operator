package controller

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrl "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/zncdatadev/secret-operator/internal/util/version"
	"github.com/zncdatadev/secret-operator/pkg/server"
)

const (
	DefaultControllerName = "secrets.kubedoop.dev"
)

var (
	logger = ctrl.Log.WithName("csi-controller")
)

type CsiController struct {
	endpoint string
	name     string
	server   server.NonBlockingServer

	client client.Client
}

func NewCsiController(
	endpoint string,
	client client.Client,
) *CsiController {
	srv := server.NewNonBlockingServer(endpoint)

	return &CsiController{
		name:     DefaultControllerName,
		endpoint: endpoint,
		server:   srv,
		client:   client,
	}
}

func (d *CsiController) Run(ctx context.Context) error {

	logger.V(1).Info("csi controller information", "versionInfo", version.NewAppInfo(d.name).String())

	cs := NewControllerServer(d.client)

	// Register the services with the gRPC server
	d.server.RegisterService(nil, nil, cs)

	if err := d.server.Start(ctx); err != nil {
		return err
	}

	// Gracefully stop the server when the context is done
	go func() {
		<-ctx.Done()
		d.server.Stop()
	}()

	d.server.Wait()
	logger.Info("csi controller stopped")
	return nil
}

func (d *CsiController) Stop() {
	d.server.Stop()
}
