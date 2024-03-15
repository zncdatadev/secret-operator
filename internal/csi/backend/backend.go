package backend

import "context"

type Backend interface {
	GetSecretData(ctx context.Context) (map[string]string, error)
}

type BaseBackend struct {
}

func NewBaseBackend() Backend {
	panic("unimplemented")
}
