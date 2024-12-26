package backend

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/zncdatadev/operator-go/pkg/constants"
	secretsv1alpha1 "github.com/zncdatadev/secret-operator/api/v1alpha1"
	"github.com/zncdatadev/secret-operator/pkg/pod_info"
	"github.com/zncdatadev/secret-operator/pkg/util"
	"github.com/zncdatadev/secret-operator/pkg/volume"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ IBackend = &K8sSearchBackend{}

type K8sSearchBackend struct {
	client          client.Client
	podInfo         *pod_info.PodInfo
	volumeContext   *volume.SecretVolumeContext
	searchNamespace *secretsv1alpha1.SearchNamespaceSpec
}

func NewK8sSearchBackend(config *BackendConfig) (IBackend, error) {
	spec := config.SecretClass.Spec.Backend.K8sSearch
	if spec == nil {
		return nil, errors.New("k8sSearchSpec is nil in secret class")
	}

	if spec.SearchNamespace == nil {
		return nil, errors.New("searchNamespace is nil in secret class")
	}

	return &K8sSearchBackend{
		client:          config.Client,
		podInfo:         config.PodInfo,
		volumeContext:   config.VolumeContext,
		searchNamespace: spec.SearchNamespace,
	}, nil
}

func (k *K8sSearchBackend) getPod() *corev1.Pod {
	return k.podInfo.Pod
}

func (k *K8sSearchBackend) namespace() (*string, error) {
	if k.searchNamespace == nil {
		return nil, errors.New("searchNamespace is nil")
	}

	if k.searchNamespace.Pod != nil {
		ns := k.getPod().GetNamespace()
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
		constants.AnnotationSecretsClass: k.volumeContext.Class,
	}

	scope := k.volumeContext.Scope
	pod := k.getPod()

	if scope.Pod != "" {
		labels[constants.LabelSecretsPod] = pod.GetName()
	}

	if scope.Node != "" {
		labels[constants.LabelSecretsNode] = pod.Spec.NodeName
	}

	if scope.Services != nil {
		labels[constants.LabelSecretsService] = strings.Join(scope.Services, ",")
	}

	return labels
}

func (k *K8sSearchBackend) GetQualifiedNodeNames(ctx context.Context) ([]string, error) {
	panic("implement me")
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
