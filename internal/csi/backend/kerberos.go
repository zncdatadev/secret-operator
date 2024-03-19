package backend

import "context"

type KerberosBackend struct {
}

// GetSecretData implements Backend.
func (k *KerberosBackend) GetSecretData(ctx context.Context) (map[string]string, error) {
	panic("unimplemented")
}
