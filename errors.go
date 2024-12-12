package redis

import (
	"errors"
)

var (
	ErrKeyNotFound      = errors.New("key not found")
	ErrInvalidFieldType = errors.New("invalid field type")
)
