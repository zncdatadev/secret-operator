package csi

import (
	"context"
	"errors"

	"k8s.io/utils/mount"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrl "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/zncdatadev/secret-operator/internal/util/version"
	"github.com/zncdatadev/secret-operator/pkg/server"
)

const (
	DefaultDriverName = "secrets.kubedoop.dev"
)

var (
	logger = ctrl.Log.WithName("csi-driver")
)

type Driver struct {
	name     string
	nodeID   string
	endpoint string

	server server.NonBlockingServer

	client client.Client
}

func NewDriver(
	nodeID string,
	endpoint string,
	client client.Client,
) *Driver {
	srv := server.NewNonBlockingServer(endpoint)

	return &Driver{
		name:     DefaultDriverName,
		nodeID:   nodeID,
		endpoint: endpoint,
		server:   srv,
		client:   client,
	}
}

func (d *Driver) Run(ctx context.Context) error {

	logger.V(1).Info("csi node driver information", "versionInfo", version.NewAppInfo(d.name).String())

	// check node id
	if d.nodeID == "" {
		return errors.New("NodeID is not provided")
	}

	ns := NewNodeServer(d.nodeID, mount.New("secret-csi"), d.client)

	is := NewIdentityServer(d.name, version.BuildVersion)

	// Register the services with the gRPC server
	d.server.RegisterService(ns, is, nil)

	if err := d.server.Start(ctx); err != nil {
		return err
	}

	// Gracefully stop the server when the context is done
	go func() {
		<-ctx.Done()
		d.server.Stop()
	}()

	d.server.Wait()
	logger.Info("csi driver stopped")
	return nil
}

func (d *Driver) Stop() {
	d.server.Stop()
}
