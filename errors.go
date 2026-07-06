package xredis

import "errors"

var (
	ErrKeyNotFound       = errors.New("key not found")
	ErrInvalidFieldType  = errors.New("invalid field type")
	ErrInvalidHashObject = errors.New("invalid hash object")
	ErrInvalidTTL        = errors.New("invalid ttl")
	ErrInvalidConfig     = errors.New("invalid redis config")
)
