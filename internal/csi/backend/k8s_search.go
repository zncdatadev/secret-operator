package backend

import "context"

var _ Backend = &K8sSearchBackend{}

type K8sSearchBackend struct {
	BaseBackend
}

// GetSecretData implements Backend.
func (k *K8sSearchBackend) GetSecretData(ctx context.Context) (map[string]string, error) {
	panic("unimplemented")
}
