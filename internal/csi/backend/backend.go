package backend

import (
	"context"

	secretsv1alpha1 "github.com/zncdata-labs/secret-operator/api/v1alpha1"
	"github.com/zncdata-labs/secret-operator/internal/csi/util"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type IBackend interface {
	GetSecretData(ctx context.Context) (map[string]string, error)
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
	secretClass := &secretsv1alpha1.SecretClass{}

	if secretClass.Spec.Backend.Kerberos != nil {
		return &KerberosBackend{}
	}

	if secretClass.Spec.Backend.AutoTls != nil {
		return &AutoTlsBackend{}
	}

	if secretClass.Spec.Backend.K8sSearch != nil {
		panic("not implemented")
	}

	panic("can not find backend")
}

func (b *Backend) getSecretFromImpl(ctx context.Context, impl IBackend) (map[string]string, error) {
	return impl.GetSecretData(ctx)
}

func (b *Backend) GetSecretData(ctx context.Context) (map[string]string, error) {

	impl := b.backendImpl()

	return b.getSecretFromImpl(ctx, impl)
}
