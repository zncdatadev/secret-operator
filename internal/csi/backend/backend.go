package backend

import (
	"context"

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

type IBackend interface {
	GetSecretData(ctx context.Context) (*util.SecretContent, error)
}

type Backend struct {
	client         client.Client
	podInfo        *pod_info.PodInfo
	volumeSelector *volume.SecretVolumeSelector
	secretClass    *secretsv1alpha1.SecretClass
}

func NewBackend(
	Client client.Client,
	PodInfo *pod_info.PodInfo,
	VolumeSelector *volume.SecretVolumeSelector,
	secretClass *secretsv1alpha1.SecretClass,
) *Backend {
	return &Backend{
		client:         Client,
		podInfo:        PodInfo,
		volumeSelector: VolumeSelector,
		secretClass:    secretClass,
	}
}

func (b *Backend) backendImpl() (IBackend, error) {

	backend := b.secretClass.Spec.Backend

	if backend.Kerberos != nil {
		panic("not implemented")
	}

	if backend.AutoTls != nil {
		return NewAutoTlsBackend(
			b.client,
			b.podInfo,
			b.volumeSelector,
			backend.AutoTls,
		)
	}

	if backend.K8sSearch != nil {
		return NewK8sSearchBackend(
			b.client,
			b.podInfo,
			b.volumeSelector,
			backend.K8sSearch,
		)
	}

	panic("can not find backend")
}

func (b *Backend) GetSecretData(ctx context.Context) (*util.SecretContent, error) {
	impl, err := b.backendImpl()
	if err != nil {
		return nil, err
	}

	return impl.GetSecretData(ctx)
}
