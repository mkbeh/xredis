package xredis

import "errors"

var (
	// ErrKeyNotFound is returned when a Redis key or cache value is not found.
	ErrKeyNotFound = errors.New("key not found")

	// ErrInvalidFieldType is returned when a struct field has an unsupported type.
	ErrInvalidFieldType = errors.New("invalid field type")

	// ErrInvalidHashObject is returned when a value cannot be used as a Redis hash object.
	ErrInvalidHashObject = errors.New("invalid hash object")

	// ErrInvalidTTL is returned when a TTL or duration option is invalid.
	ErrInvalidTTL = errors.New("invalid ttl")

	// ErrInvalidConfig is returned when Redis client configuration is invalid.
	ErrInvalidConfig = errors.New("invalid redis config")

	// ErrInvalidCache is returned when a typed cache is invalid or misconfigured.
	ErrInvalidCache = errors.New("invalid cache")

	// ErrInvalidCacheEntry is returned when a cached Redis entry has an invalid format.
	ErrInvalidCacheEntry = errors.New("invalid cache entry")

	// ErrInvalidCacheLoader is returned when a cache loader is nil or invalid.
	ErrInvalidCacheLoader = errors.New("invalid cache loader")

	// ErrLockNotAcquired is returned when a Redis lock cannot be acquired.
	ErrLockNotAcquired = errors.New("lock not acquired")

	// ErrLockNotOwned is returned when a lock is expired, deleted, or owned by another token.
	ErrLockNotOwned = errors.New("lock not owned")

	// ErrInvalidLock is returned when a lock, lock key, owner token, or client is invalid.
	ErrInvalidLock = errors.New("invalid lock")

	// ErrInvalidRateLimiter is returned when a rate limiter is invalid or misconfigured.
	ErrInvalidRateLimiter = errors.New("invalid rate limiter")

	// ErrInvalidRateLimit is returned when rate limit configuration is invalid.
	ErrInvalidRateLimit = errors.New("invalid rate limit")
)
