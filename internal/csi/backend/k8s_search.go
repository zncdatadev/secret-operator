package backend

import (
	"context"
	"errors"

	secretsv1alpha1 "github.com/zncdata-labs/secret-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type K8sSearchBackend struct {
	client      client.Client
	secretClass *secretsv1alpha1.SecretClass
	pod         *corev1.Pod
}

func NewK8sSearchBackend(
	client client.Client,
	secretClass *secretsv1alpha1.SecretClass,
	pod *corev1.Pod,
) *K8sSearchBackend {

	return &K8sSearchBackend{
		client:      client,
		secretClass: secretClass,
		pod:         pod,
	}
}

func (k *K8sSearchBackend) namespace(searchNamespace *secretsv1alpha1.SearchNamespaceSpec) (*string, error) {
	if searchNamespace == nil {
		return nil, errors.New("searchNamespace is nil")
	}

	if searchNamespace.Name != nil {
		return searchNamespace.Name, nil
	}

	if searchNamespace.Pod != nil {
		ns := k.pod.GetNamespace()
		return &ns, nil
	}

	return nil, errors.New("can not found namespace name in searchNamespace field")

}

func (k *K8sSearchBackend) getSecret(ctx context.Context, namespace string) (*corev1.SecretList, error) {
	objs := &corev1.SecretList{}

	err := k.client.List(
		ctx,
		objs,
		client.InNamespace(namespace),
		client.MatchingLabels(
			map[string]string{},
		),
	)

	if err != nil {
		return nil, err
	}

	return objs, nil

}

// GetSecretData implements Backend.
func (k *K8sSearchBackend) GetSecretData(ctx context.Context) (map[string]string, error) {
	panic("unimplemented")
}
