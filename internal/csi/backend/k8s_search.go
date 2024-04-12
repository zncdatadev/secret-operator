package backend

import (
	"context"
	"errors"
	"fmt"
	"strings"

	secretsv1alpha1 "github.com/zncdata-labs/secret-operator/api/v1alpha1"
	"github.com/zncdata-labs/secret-operator/pkg/pod_info"
	"github.com/zncdata-labs/secret-operator/pkg/util"
	"github.com/zncdata-labs/secret-operator/pkg/volume"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type K8sSearchBackend struct {
	client          client.Client
	podInfo         *pod_info.PodInfo
	volumeSelector  *volume.SecretVolumeSelector
	searchNamespace *secretsv1alpha1.SearchNamespaceSpec
}

func NewK8sSearchBackend(
	client client.Client,
	podInfo *pod_info.PodInfo,
	volumeSelector *volume.SecretVolumeSelector,
	k8sSearchSpec *secretsv1alpha1.K8sSearchSpec,
) (*K8sSearchBackend, error) {

	if k8sSearchSpec == nil {
		return nil, errors.New("k8sSearchSpec is nil in secret class")
	}

	if k8sSearchSpec.SearchNamespace == nil {
		return nil, errors.New("searchNamespace is nil in secret class")
	}

	return &K8sSearchBackend{
		client:          client,
		podInfo:         podInfo,
		volumeSelector:  volumeSelector,
		searchNamespace: k8sSearchSpec.SearchNamespace,
	}, nil
}

func (k *K8sSearchBackend) GetPod() *corev1.Pod {
	return k.podInfo.Pod
}

func (k *K8sSearchBackend) namespace() (*string, error) {
	if k.searchNamespace == nil {
		return nil, errors.New("searchNamespace is nil")
	}

	if k.searchNamespace.Pod != nil {
		ns := k.GetPod().GetNamespace()
		return &ns, nil
	}

	if k.searchNamespace.Name != nil {
		return k.searchNamespace.Name, nil
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

	logger.V(5).Info("found secret total, use first", "total", len(objs.Items), "secret", secret.Name, "namespace", secret.Namespace)

	return secret, nil
}

// matchingLabels returns the labels that should be used to search for the secret.
// The labels are based on the secret class and the volume selector.
func (k *K8sSearchBackend) matchingLabels() map[string]string {
	labels := map[string]string{
		volume.SecretsZncdataClass: k.volumeSelector.Class,
	}

	scope := k.volumeSelector.Scope
	pod := k.GetPod()

	if scope.Pod != "" {
		labels[volume.SecretsZncdataPod] = pod.GetName()
	}

	if scope.Node != "" {
		labels[volume.SecretsZncdataNodeName] = pod.Spec.NodeName
	}

	if scope.Services != nil {
		labels[volume.SecretsZncdataService] = strings.Join(scope.Services, ",")
	}

	// TODO: add listener label when listener volume is supported

	return labels
}

// GetSecretData implements Backend.
func (k *K8sSearchBackend) GetSecretData(ctx context.Context) (*util.SecretContent, error) {

	namespace, err := k.namespace()
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
