package csi

import (
	"context"
	"errors"

	"github.com/zncdata-labs/secret-operator/internal/csi/version"
	"k8s.io/utils/mount"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	DefaultDriverName = "secret.zncdata.dev"
)

type Driver struct {
	name     string
	nodeID   string
	endpoint string

	server NonBlockingServer

	client client.Client
}

func NewDriver(
	name string,
	nodeID string,
	endpoint string,
	client client.Client,
) *Driver {
	srv := NewNonBlockingServer()

	return &Driver{
		name:     name,
		nodeID:   nodeID,
		endpoint: endpoint,
		server:   srv,
		client:   client,
	}
}

func (d *Driver) Run(ctx context.Context, testMode bool) error {

	log.V(1).Info("Driver information", "versionInfo", version.GetVersion(d.name))

	// check node id
	if d.nodeID == "" {
		return errors.New("NodeID is not provided")
	}

	ns := NewNodeServer(
		d.nodeID,
		mount.New(""),
		d.client,
	)

	is := NewIdentityServer(d.name, version.BuildVersion)
	cs := NewControllerServer(d.client)

	d.server.Start(d.endpoint, is, cs, ns, testMode)

	// Gracefully stop the server when the context is done
	go func() {
		<-ctx.Done()
		d.server.Stop()
	}()

	d.server.Wait()
	log.Info("Server stopped")
	return nil
}

func (d *Driver) Stop() {
	d.server.Stop()
}
