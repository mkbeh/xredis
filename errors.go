package xredis

import "errors"

var (
	// ErrKeyNotFound is returned when a Redis key or cache value is not found.
	ErrKeyNotFound = errors.New("key not found")

	// ErrInvalidHashObject is returned when a value cannot be used as a Redis hash object.
	ErrInvalidHashObject = errors.New("invalid hash object")

	// ErrInvalidTTL is returned when a TTL or duration option is invalid.
	ErrInvalidTTL = errors.New("invalid ttl")

	// ErrInvalidConfig is returned when Redis client configuration is invalid.
	ErrInvalidConfig = errors.New("invalid redis config")

	// ErrInvalidCache is returned when a typed cache is invalid or misconfigured.
	ErrInvalidCache = errors.New("invalid cache")

	// ErrInvalidCacheLoader is returned when a cache loader is nil or invalid.
	ErrInvalidCacheLoader = errors.New("invalid cache loader")

	// ErrInvalidCacheMarker is returned when a cache negative marker is invalid.
	ErrInvalidCacheMarker = errors.New("invalid cache negative marker")

	// ErrCacheLoad is returned when a cache loader fails.
	ErrCacheLoad = errors.New("cache load failed")

	// ErrLockNotOwned is returned when a lock is expired, deleted, or owned by another token.
	ErrLockNotOwned = errors.New("lock not owned")

	// ErrInvalidLock is returned when a lock, lock key, owner token, or client is invalid.
	ErrInvalidLock = errors.New("invalid lock")

	// ErrInvalidRateLimiter is returned when a rate limiter is invalid or misconfigured.
	ErrInvalidRateLimiter = errors.New("invalid rate limiter")

	// ErrInvalidRateLimit is returned when rate limit configuration is invalid.
	ErrInvalidRateLimit = errors.New("invalid rate limit")

	// ErrInvalidScan is returned when scan options or handler are invalid.
	ErrInvalidScan = errors.New("invalid scan")

	// ErrInvalidPipeline is returned when pipeline input or configuration is invalid.
	ErrInvalidPipeline = errors.New("invalid pipeline")

	// ErrInvalidVersionedStore is returned when a versioned store is invalid or misconfigured.
	ErrInvalidVersionedStore = errors.New("invalid versioned store")

	// ErrInvalidEntry is returned when a stored Redis entry has an invalid internal representation.
	ErrInvalidEntry = errors.New("invalid entry")

	// ErrUnsupportedType is returned when a typed component is created with an
	// unsupported value type.
	ErrUnsupportedType = errors.New("unsupported type")
)
