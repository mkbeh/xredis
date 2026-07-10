package xredis

import (
	"context"
	"errors"
	"math/rand/v2"
	"time"

	rdb "github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

// Cache entries are stored with a one-byte marker prefix.
//
// Format:
//
//	[0]       cached not-found result
//	[1] data  cached encoded value
//
// The marker is not part of the encoded value payload.
// It allows Cache to distinguish regular values from cached not-found results
// regardless of the configured Codec.
const (
	cacheNegativeMarker byte = 0
	cacheValueMarker    byte = 1
)

// cacheState represents the status of a cache entry read from Redis.
type cacheState uint8

const (
	// cacheMiss indicates that the key does not exist in Redis.
	cacheMiss cacheState = iota

	// cacheHit indicates that the key exists and contains a regular value entry.
	cacheHit

	// cacheNegative indicates that the key exists and represents a cached not-found result.
	cacheNegative
)

// cacheLoadResult preserves the concrete generic type when a loaded value is
// passed through singleflight as any.
type cacheLoadResult[T any] struct {
	value T
}

// Loader loads a value when it is missing from cache.
//
// On success, it should return a value and nil error.
// If the value is not found, it should return ErrKeyNotFound or an error matched
// by WithCacheNotFound. Other errors are returned to the caller and are not
// cached by default.
type Loader[T any] func(ctx context.Context) (T, error)

// Cache is a typed Redis cache.
//
// Values returned by Cache are not deep-copied.
// If T is a pointer or contains reference fields such as slices or maps,
// callers should treat returned values as immutable or clone them before mutation.
type Cache[T any] struct {
	client *Client

	prefix      string
	ttl         time.Duration
	jitter      time.Duration
	negativeTTL time.Duration

	codec      Codec
	isNotFound func(error) bool

	group singleflight.Group
}

// CacheOption configures typed cache.
type CacheOption func(*cacheOptions)

type cacheOptions struct {
	prefix      string
	ttl         time.Duration
	jitter      time.Duration
	negativeTTL time.Duration

	codec      Codec
	isNotFound func(error) bool
}

// NewCache creates a typed Redis cache.
func NewCache[T any](client *Client, opts ...CacheOption) (*Cache[T], error) {
	options := cacheOptions{
		codec:      JSONCodec{},
		isNotFound: defaultCacheIsNotFound,
	}

	if client != nil && client.codec != nil {
		options.codec = client.codec
	}

	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}

	if err := validateCacheOptions(client, options); err != nil {
		return nil, err
	}

	return &Cache[T]{
		client:      client,
		prefix:      options.prefix,
		ttl:         options.ttl,
		jitter:      options.jitter,
		negativeTTL: options.negativeTTL,
		codec:       options.codec,
		isNotFound:  options.isNotFound,
	}, nil
}

func validateCacheOptions(client *Client, opts cacheOptions) error {
	if client == nil || client.conn == nil {
		return ErrInvalidCache
	}

	if opts.ttl < 0 || opts.jitter < 0 || opts.negativeTTL < 0 {
		return ErrInvalidTTL
	}

	return nil
}

// WithCachePrefix configures cache key prefix.
func WithCachePrefix(prefix string) CacheOption {
	return func(opts *cacheOptions) {
		opts.prefix = prefix
	}
}

// WithCacheTTL configures default cache TTL.
func WithCacheTTL(ttl time.Duration) CacheOption {
	return func(opts *cacheOptions) {
		opts.ttl = ttl
	}
}

// WithCacheJitter configures random TTL spread.
//
// Jitter is added to positive TTL values to reduce mass expiration.
func WithCacheJitter(jitter time.Duration) CacheOption {
	return func(opts *cacheOptions) {
		opts.jitter = jitter
	}
}

// WithCacheNegativeTTL configures TTL for cached not-found results.
//
// Negative caching is disabled by default.
// Set ttl > 0 to cache not-found results.
// Regular loader errors are never cached unless isNotFound matches them.
func WithCacheNegativeTTL(ttl time.Duration) CacheOption {
	return func(opts *cacheOptions) {
		opts.negativeTTL = ttl
	}
}

// WithCacheCodec configures cache value codec.
func WithCacheCodec(codec Codec) CacheOption {
	return func(opts *cacheOptions) {
		if codec != nil {
			opts.codec = codec
		}
	}
}

// WithCacheNotFound configures not-found error detection for negative caching.
//
// The function should return true only for stable not-found errors.
// Transient errors such as timeouts, canceled contexts, or connection errors
// should not be treated as not-found.
func WithCacheNotFound(fn func(error) bool) CacheOption {
	return func(opts *cacheOptions) {
		if fn != nil {
			opts.isNotFound = fn
		}
	}
}

// Get reads a typed value from cache.
func (c *Cache[T]) Get(ctx context.Context, key string) (T, bool, error) {
	value, state, err := c.get(ctx, key)
	if err != nil {
		var zero T
		return zero, false, err
	}

	if state != cacheHit {
		var zero T
		return zero, false, nil
	}

	return value, true, nil
}

// GetOrLoad reads a value from cache or loads it using loader.
//
// Concurrent calls for the same key share one loader execution through singleflight.
// The shared loader uses the context from the call that starts that execution.
// Each caller can stop waiting when its own context is canceled.
//
// If shared loading should outlive request cancellation, pass an independent
// context to GetOrLoad.
//
// Values returned by loader may be shared between concurrent callers for the
// same key. Treat mutable values as read-only or clone them before mutation.
func (c *Cache[T]) GetOrLoad(ctx context.Context, key string, loader Loader[T]) (T, error) {
	var zero T

	if loader == nil {
		return zero, ErrInvalidCacheLoader
	}

	value, state, err := c.get(ctx, key)
	if err != nil {
		return zero, err
	}

	switch state {
	case cacheHit:
		return value, nil
	case cacheNegative:
		return zero, ErrKeyNotFound
	case cacheMiss: // proceed to singleflight loading
	}

	ch := c.group.DoChan(c.key(key), func() (any, error) {
		value, err := c.load(ctx, key, loader)
		return cacheLoadResult[T]{
			value: value,
		}, err
	})

	select {
	case <-ctx.Done():
		return zero, ctx.Err()

	case result := <-ch:
		if result.Err != nil {
			return zero, result.Err
		}

		loaded, ok := result.Val.(cacheLoadResult[T])
		if !ok {
			return zero, ErrInvalidCacheEntry
		}

		return loaded.value, nil
	}
}

// Set stores a typed value in cache using default TTL.
func (c *Cache[T]) Set(ctx context.Context, key string, value T) error {
	data, err := c.encode(value)
	if err != nil {
		return err
	}

	return c.client.conn.Set(ctx, c.key(key), data, c.expiration(c.ttl)).Err()
}

// Delete removes a value from cache.
func (c *Cache[T]) Delete(ctx context.Context, key string) error {
	return c.client.conn.Del(ctx, c.key(key)).Err()
}

// Forget removes an in-flight loader for the key from singleflight.
func (c *Cache[T]) Forget(key string) {
	c.group.Forget(c.key(key))
}

// CompareAndDelete deletes a cached value only when it equals expected.
//
// It returns deleted=false when the key is missing, contains a negative cache
// entry, or contains a different value.
func (c *Cache[T]) CompareAndDelete(ctx context.Context, key string, expected T) (bool, error) {
	expectedData, err := c.encode(expected)
	if err != nil {
		return false, err
	}

	return c.client.compareAndDelete(
		ctx,
		c.key(key),
		expectedData,
	)
}

// CompareAndSwap replaces a cached value only when it equals expected.
//
// A successful swap refreshes the cache entry expiration using the configured
// cache TTL and jitter.
func (c *Cache[T]) CompareAndSwap(ctx context.Context, key string, expected, value T) (bool, error) {
	expectedData, err := c.encode(expected)
	if err != nil {
		return false, err
	}

	valueData, err := c.encode(value)
	if err != nil {
		return false, err
	}

	return c.client.compareAndSwap(
		ctx,
		c.key(key),
		expectedData,
		valueData,
		c.expiration(c.ttl),
	)
}

func (c *Cache[T]) load(ctx context.Context, key string, loader Loader[T]) (T, error) {
	value, err := loader(ctx)
	if err != nil {
		if c.negativeTTL > 0 && c.isNotFound(err) {
			if setErr := c.setNegative(ctx, key); setErr != nil {
				return value, errors.Join(err, setErr)
			}
		}

		return value, err
	}

	if err = c.Set(ctx, key, value); err != nil {
		return value, err
	}

	return value, nil
}

func (c *Cache[T]) get(ctx context.Context, key string) (T, cacheState, error) {
	var zero T

	data, err := c.client.conn.Get(ctx, c.key(key)).Bytes()
	if err != nil {
		if errors.Is(err, rdb.Nil) {
			return zero, cacheMiss, nil
		}

		return zero, cacheMiss, err
	}

	if len(data) == 0 {
		return zero, cacheMiss, ErrInvalidCacheEntry
	}

	switch data[0] {
	case cacheNegativeMarker:
		if len(data) != 1 {
			return zero, cacheMiss, ErrInvalidCacheEntry
		}

		return zero, cacheNegative, nil

	case cacheValueMarker:
		value, err := c.decode(data[1:])
		if err != nil {
			return zero, cacheMiss, err
		}

		return value, cacheHit, nil

	default:
		return zero, cacheMiss, ErrInvalidCacheEntry
	}
}

func (c *Cache[T]) setNegative(ctx context.Context, key string) error {
	if c.negativeTTL <= 0 {
		return nil
	}

	ttl := c.expiration(c.negativeTTL)

	return c.client.conn.Set(ctx, c.key(key), []byte{cacheNegativeMarker}, ttl).Err()
}

func (c *Cache[T]) encode(value T) ([]byte, error) {
	data, err := c.codec.Marshal(value)
	if err != nil {
		return nil, err
	}

	out := make([]byte, len(data)+1)
	out[0] = cacheValueMarker
	copy(out[1:], data)

	return out, nil
}

func (c *Cache[T]) decode(data []byte) (T, error) {
	var value T

	if err := c.codec.Unmarshal(data, &value); err != nil {
		return value, err
	}

	return value, nil
}

func (c *Cache[T]) key(key string) string {
	return c.prefix + key
}

func (c *Cache[T]) expiration(ttl time.Duration) time.Duration {
	if ttl == 0 || c.jitter == 0 {
		return ttl
	}

	return ttl + rand.N(c.jitter)
}

func defaultCacheIsNotFound(err error) bool {
	return errors.Is(err, ErrKeyNotFound) || errors.Is(err, rdb.Nil)
}
