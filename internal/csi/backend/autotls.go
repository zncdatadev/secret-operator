package backend

import "context"

type AutoTlsBackend struct {
}

// GetSecretData implements Backend.
func (a *AutoTlsBackend) GetSecretData(ctx context.Context) (map[string]string, error) {
	panic("unimplemented")
}
