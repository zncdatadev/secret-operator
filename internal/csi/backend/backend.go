package backend

import (
	"context"

	secretsv1alpha1 "github.com/zncdata-labs/secret-operator/api/v1alpha1"
	"github.com/zncdata-labs/secret-operator/internal/csi/util"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type IBackend interface {
	GetSecretData(ctx context.Context) (*util.SecretContent, error)
}

type Backend struct {
	client        client.Client
	secretClass   *secretsv1alpha1.SecretClass
	pod           *corev1.Pod
	volumeContext *util.VolumeContextSpec
}

func NewBackend(
	client client.Client,
	secretClass *secretsv1alpha1.SecretClass,
	pod *corev1.Pod,
	volumeContext *util.VolumeContextSpec,
) *Backend {
	return &Backend{
		client:        client,
		secretClass:   secretClass,
		pod:           pod,
		volumeContext: volumeContext,
	}
}

func (b *Backend) backendImpl() IBackend {
	backend := b.secretClass.Spec.Backend

	if backend.Kerberos != nil {
		panic("not implemented")
	}

	if backend.AutoTls != nil {
		panic("not implemented")
	}

	if backend.K8sSearch != nil {
		return &K8sSearchBackend{
			client:        b.client,
			secretClass:   b.secretClass,
			pod:           b.pod,
			volumeContext: b.volumeContext,
		}
	}

	panic("can not find backend")
}

func (b *Backend) GetSecretData(ctx context.Context) (*util.SecretContent, error) {

	impl := b.backendImpl()

	return impl.GetSecretData(ctx)
}
