package backend

import (
	"context"
	"errors"
	"fmt"
	"strings"

	secretsv1alpha1 "github.com/zncdata-labs/secret-operator/api/v1alpha1"
	"github.com/zncdata-labs/secret-operator/internal/csi/util"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type K8sSearchBackend struct {
	client        client.Client
	secretClass   *secretsv1alpha1.SecretClass
	pod           *corev1.Pod
	volumeContext *util.VolumeContextSpec
}

func NewK8sSearchBackend(
	client client.Client,
	secretClass *secretsv1alpha1.SecretClass,
	pod *corev1.Pod,
	volumeContext *util.VolumeContextSpec,
) *K8sSearchBackend {

	return &K8sSearchBackend{
		client:        client,
		secretClass:   secretClass,
		pod:           pod,
		volumeContext: volumeContext,
	}
}

func (k *K8sSearchBackend) namespace(searchNamespace *secretsv1alpha1.SearchNamespaceSpec) (*string, error) {
	if searchNamespace == nil {
		return nil, errors.New("searchNamespace is nil")
	}

	if searchNamespace.Pod != nil {
		ns := k.pod.GetNamespace()
		return &ns, nil
	}

	if searchNamespace.Name != nil {
		return searchNamespace.Name, nil
	}

	return nil, errors.New("can not found namespace name in searchNamespace field")
}

// GetSecretData implements Backend.
func (k *K8sSearchBackend) getSecret(
	ctx context.Context,
	namespace string,
	matchingLabels map[string]string,
) (*corev1.Secret, error) {
	objs := &corev1.SecretList{}

	err := k.client.List(
		ctx,
		objs,
		client.InNamespace(namespace),
		client.MatchingLabels(matchingLabels),
	)
	if err != nil {
		return nil, err
	}

	if len(objs.Items) == 0 {
		return nil, fmt.Errorf("can not found secret in namespace %s with labels: %v", namespace, matchingLabels)
	}

	secret := &objs.Items[0]

	log.V(5).Info("found secret total, use first", "total", len(objs.Items), "secret", secret.Name, "namespace", secret.Namespace)

	return secret, nil
}

func (k *K8sSearchBackend) scopes() *[]string {
	scopesStr := k.volumeContext.Scope
	if scopesStr == nil {
		return nil
	}
	splitedScopes := strings.Split(*scopesStr, ",")
	return &splitedScopes
}

func (k *K8sSearchBackend) matchingLabels() map[string]string {
	labels := map[string]string{
		util.SECRETS_ZNCDATA_CLASS: k.secretClass.Name,
	}

	scopes := k.scopes()

	if scopes != nil {
		for _, scope := range *scopes {
			switch scope {
			case util.SECRETS_SCOPE_NODE:
				labels[util.SECRETS_ZNCDATA_NODE] = k.pod.Spec.NodeName
			case util.SECRETS_SCOPE_POD:
				labels[util.SECRETS_ZNCDATA_POD] = k.pod.GetName()
			}
		}
	}

	return labels
}

// GetSecretData implements Backend.
func (k *K8sSearchBackend) GetSecretData(ctx context.Context) (*util.SecretContent, error) {

	namespace, err := k.namespace(k.secretClass.Spec.Backend.K8sSearch.SearchNamespace)
	if err != nil {
		return nil, err
	}

	matchingLabels := k.matchingLabels()

	secret, err := k.getSecret(ctx, *namespace, matchingLabels)
	if err != nil {
		return nil, err
	}

	decoded, err := DecodeSecretData(secret.Data)
	if err != nil {
		return nil, err
	}

	return &util.SecretContent{
		Data: decoded,
	}, nil
}

// DecodeSecretData decodes the secret data.
// secret data is base64 encoded.
func DecodeSecretData(data map[string][]byte) (map[string]string, error) {
	decoded := make(map[string]string)
	for k, v := range data {
		decoded[k] = string(v)
	}
	return decoded, nil
}
