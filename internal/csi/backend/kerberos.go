package backend

import "context"

var _ Backend = &KerberosBackend{}

type KerberosBackend struct {
	BaseBackend
}

// GetSecretData implements Backend.
func (k *KerberosBackend) GetSecretData(ctx context.Context) (map[string]string, error) {
	panic("unimplemented")
}
