package backend

import (
	"context"
	"fmt"
	"sync"

	secretsv1alpha1 "github.com/zncdatadev/secret-operator/api/v1alpha1"
	"github.com/zncdatadev/secret-operator/pkg/pod_info"
	"github.com/zncdatadev/secret-operator/pkg/util"
	"github.com/zncdatadev/secret-operator/pkg/volume"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	logger = ctrl.Log.WithName("csi-backend")
)

type BackendType string

const (
	KerberosKeytabType BackendType = "KerberosKeytab"
	AutoTlsType        BackendType = "AutoTls"
	K8sSearchType      BackendType = "K8sSearch"
)

type BackendConfig struct {
	Client        client.Client
	PodInfo       *pod_info.PodInfo
	VolumeContext *volume.SecretVolumeContext
	SecretClass   *secretsv1alpha1.SecretClass
}

type IBackend interface {
	GetSecretData(ctx context.Context) (*util.SecretContent, error)
	GetQualifiedNodeNames(ctx context.Context) ([]string, error)
}

type Backend struct {
	impl IBackend
}

func NewBackend(ctx context.Context, c client.Client, podInfo *pod_info.PodInfo, volumeCtx *volume.SecretVolumeContext) (*Backend, error) {
	secretClass := &secretsv1alpha1.SecretClass{}
	if err := c.Get(ctx, client.ObjectKey{Name: volumeCtx.Class}, secretClass); err != nil {
		return nil, err
	}

	config := &BackendConfig{Client: c, PodInfo: podInfo, VolumeContext: volumeCtx, SecretClass: secretClass}

	backendType := determineBackendType(config.SecretClass)
	impl, err := CreateBackend(backendType, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create backend: %w", err)
	}
	return &Backend{impl: impl}, nil
}

func determineBackendType(secretClass *secretsv1alpha1.SecretClass) BackendType {
	backend := secretClass.Spec.Backend

	if backend.KerberosKeytab != nil {
		return KerberosKeytabType
	}
	if backend.AutoTls != nil {
		return AutoTlsType
	}
	if backend.K8sSearch != nil {
		return K8sSearchType
	}
	return ""
}

func (b *Backend) GetSecretData(ctx context.Context) (*util.SecretContent, error) {
	return b.impl.GetSecretData(ctx)
}

func (b *Backend) GetQualifiedNodeNames(ctx context.Context) ([]string, error) {
	return b.impl.GetQualifiedNodeNames(ctx)
}

type BackendFactory func(config *BackendConfig) (IBackend, error)

type BackendRegistry struct {
	mu        sync.RWMutex
	factories map[BackendType]BackendFactory
}

var registry = &BackendRegistry{factories: make(map[BackendType]BackendFactory)}

func RegisterBackend(backendType BackendType, factory BackendFactory) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.factories[backendType] = factory
}

func CreateBackend(backendType BackendType, config *BackendConfig) (IBackend, error) {
	registry.mu.RLock()
	factory, exists := registry.factories[backendType]
	registry.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("backend type %s not registered", backendType)
	}
	return factory(config)
}

func init() {
	RegisterBackend(KerberosKeytabType, NewKerberosBackend)
	RegisterBackend(AutoTlsType, NewAutoTlsBackend)
	RegisterBackend(K8sSearchType, NewK8sSearchBackend)
}
