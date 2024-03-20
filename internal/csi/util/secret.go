package util

type SecretContent struct {
	Data        map[string]string
	ExpiresTime *int64
}
