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

// +kubebuilder:rbac:groups=secrets.kubedoop.dev,resources=secretclasses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=secrets.kubedoop.dev,resources=secretclasses/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=secrets.kubedoop.dev,resources=secretclasses/finalizers,verbs=update
// +kubebuilder:rbac:groups=storage.k8s.io,resources=csidrivers,verbs=get;list;watch
// +kubebuilder:rbac:groups=storage.k8s.io,resources=storageclasses,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=persistentvolumes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=listeners.kubedoop.dev,resources=listeners,verbs=get;list;watch

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

	cs := NewControllerServer(d.client)
	ns := NewNodeServer(d.nodeID, mount.New("secret-csi"), d.client)
	is := NewIdentityServer(d.name, version.BuildVersion)

	// Register the services with the gRPC server
	d.server.RegisterService(ns, is, cs)

	if err := d.server.Start(ctx); err != nil {
		return err
	}

	// Gracefully stop the server when the context is done
	go func() {
		<-ctx.Done()
		d.server.Stop()
	}()

	if err := d.server.Wait(); err != nil {
		logger.Error(err, "error while waiting for server to finish")
		return err
	}
	logger.Info("csi driver stopped")
	return nil
}

func (d *Driver) Stop() {
	d.server.Stop()
}
