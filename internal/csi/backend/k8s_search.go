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

func (k *K8sSearchBackend) namespace() (string, error) {
	if k.searchNamespace.Name != nil {
		return *k.searchNamespace.Name, nil
	}

	if k.searchNamespace.Pod != nil {
		ns := k.getPod().GetNamespace()
		return ns, nil
	}

	return "", errors.New("can not found namespace name in searchNamespace field")
}

// GetSecretData implements Backend.
func (k *K8sSearchBackend) getSecretList(ctx context.Context, matchingLabels map[string]string) (*corev1.SecretList, error) {
	namespace, err := k.namespace()
	if err != nil {
		return nil, err
	}

	objs := &corev1.SecretList{}
	if err := k.client.List(ctx, objs, client.InNamespace(namespace), client.MatchingLabels(matchingLabels)); err != nil {
		return nil, err
	}

	if len(objs.Items) == 0 {
		return nil, fmt.Errorf("can not found secret in namespace %s with labels: %v", namespace, matchingLabels)
	}

	secretNames := make([]string, 0, len(objs.Items))
	for _, obj := range objs.Items {
		secretNames = append(secretNames, obj.GetName())
	}
	logger.V(1).Info("Found secrets", "total", len(secretNames), "secrets", secretNames)

	return objs, nil
}

// matchingLabels returns the labels that should be used to search for the secret.
// The labels are based on the secret class and the volume selector.
func (k *K8sSearchBackend) matchingLabels(ctx context.Context, hasListenerNodeScope bool) (map[string]string, error) {
	labels := map[string]string{constants.AnnotationSecretsClass: k.volumeContext.Class}

	scope := k.volumeContext.Scope
	pod := k.getPod()

	if scope.Pod != "" {
		labels[constants.LabelSecretsPod] = pod.GetName()
	}

	if scope.Services != nil {
		labels[constants.LabelSecretsService] = strings.Join(scope.Services, ",")
	}

	if scope.Node != "" || hasListenerNodeScope {
		labels[constants.LabelSecretsNode] = pod.Spec.NodeName
	}

	listenerVolumesToListenerName, err := k.podInfo.GetListenerVolumeNamesToListenerName(ctx)
	if err != nil {
		return nil, err
	}
	for idx, listenerVolume := range scope.ListenerVolumes {
		label := fmt.Sprintf("secrets.stackable.tech/listener.%d", idx+1)
		if listenerName, ok := listenerVolumesToListenerName[listenerVolume]; ok {
			labels[label] = listenerName
		}
	}

	return labels, nil
}

// GetQualifiedNodeNames implements Backend.
// It returns the node names that are qualified to access the secret.
func (k *K8sSearchBackend) GetQualifiedNodeNames(ctx context.Context) ([]string, error) {
	hasListenerNodeScope, err := k.podInfo.HasListenerNodeScope(ctx)
	if err != nil {
		return nil, err
	}

	if !hasListenerNodeScope {
		return nil, nil
	}

	matchingLabels, err := k.matchingLabels(ctx, hasListenerNodeScope)
	if err != nil {
		return nil, err
	}

	objs, err := k.getSecretList(ctx, matchingLabels)
	if err != nil {
		return nil, err
	}

	if len(objs.Items) == 0 {
		return nil, nil
	}

	ndoes := make([]string, 0, len(objs.Items))
	for _, obj := range objs.Items {
		if obj.Annotations != nil {
			if node, ok := obj.Annotations[constants.LabelSecretsNode]; ok {
				ndoes = append(ndoes, node)
			}
		}
	}
	namespace, err := k.namespace()
	if err != nil {
		return nil, err
	}
	logger.V(1).Info("Found nodes from secrets with labels when listener node scope is enabled",
		"total", len(ndoes), "nodes", ndoes, "namespace", namespace, "matchingLabels", matchingLabels,
	)
	return ndoes, nil
}

// GetSecretData implements Backend.
func (k *K8sSearchBackend) GetSecretData(ctx context.Context) (*util.SecretContent, error) {
	namespace, err := k.namespace()
	if err != nil {
		return nil, err
	}

	hasListenerNodeScope, err := k.podInfo.HasListenerNodeScope(ctx)
	if err != nil {
		return nil, err
	}

	matchingLabels, err := k.matchingLabels(ctx, hasListenerNodeScope)
	if err != nil {
		return nil, err
	}

	objs, err := k.getSecretList(ctx, matchingLabels)
	if err != nil {
		return nil, err
	}

	if len(objs.Items) == 0 {
		return nil, fmt.Errorf("can not found secret in namespace %s with labels: %v", namespace, matchingLabels)
	}

	secret := objs.Items[0]
	logger.V(1).Info("Found secret", "name", secret.GetName())

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
