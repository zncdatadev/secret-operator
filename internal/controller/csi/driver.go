package csi

import (
	"context"
	"k8s.io/utils/mount"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	DefaultDriverName = "secret.csi.zncdata.dev"
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

	versionMeta, err := GetVersionYAML(d.name)

	if err != nil {
		log.Error(err, "Failed to get driver information")
		return err
	}
	log.V(2).Info("\nDRIVER INFORMATION:\n-------------------\n%s\n\nStreaming logs below:", versionMeta)

	ns := NewNodeServer(
		d.nodeID,
		mount.New(""),
		d.client,
	)

	is := NewIdentityServer(d.name, driverVersion)
	cs := NewControllerServer()

	d.server.Start(d.endpoint, is, cs, ns, testMode)

	// Gracefully stop the server when the context is done
	go func() {
		<-ctx.Done()
		d.server.Stop()
	}()

	d.server.Wait()
	return nil
}

func (d *Driver) Stop() {
	d.server.Stop()
}
