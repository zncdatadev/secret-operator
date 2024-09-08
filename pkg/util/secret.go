package util

import "time"

type SecretContent struct {
	Data        map[string]string
	ExpiresTime *time.Time
}
