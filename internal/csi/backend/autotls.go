package backend

import "context"

var _ Backend = &AutoTlsBackend{}

type AutoTlsBackend struct {
	BaseBackend
}

// GetSecretData implements Backend.
func (a *AutoTlsBackend) GetSecretData(ctx context.Context) (map[string]string, error) {
	panic("unimplemented")
}
