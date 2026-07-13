package xredis

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net"
	"reflect"
	"time"

	rdb "github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

// cacheNegativeMarker is the reserved value used for cached not-found results.
//
// Regular cache values are stored without a marker so they can be read through
// the regular Redis client. The exact value []byte{cacheNegativeMarker} is
// reserved and must not be used as a regular cache value.
const defaultCacheNegativeMarker byte = 0

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

// Loader loads a value when it is missing from cache.
//
// On success, it should return a value and nil error.
// If the value is not found, it should return ErrKeyNotFound or an error matched
// by WithCacheNotFound. Other errors are returned to the caller and are not
// cached by default.
type Loader[T any] func(ctx context.Context) (T, error)

// Cache is a typed Redis cache.
//
// Redis-native values are encoded and decoded by go-redis. Other values are
// encoded and decoded by the configured Codec.
//
// Values returned by Cache are not deep-copied. If T is a pointer or contains
// reference fields such as slices or maps, callers should treat returned values
// as immutable or clone them before mutation.
type Cache[T any] struct {
	client *Client

	prefix         string
	ttl            time.Duration
	jitter         time.Duration
	negativeTTL    time.Duration
	negativeMarker []byte

	codec        Codec
	redisEncoded bool
	isNotFound   func(error) bool

	group singleflight.Group
}

// CacheOption configures typed cache.
type CacheOption func(*cacheOptions)

type cacheOptions struct {
	prefix         string
	ttl            time.Duration
	jitter         time.Duration
	negativeTTL    time.Duration
	negativeMarker []byte

	codec      Codec
	isNotFound func(error) bool
}

// NewCache creates a typed Redis cache.
func NewCache[T any](client *Client, opts ...CacheOption) (*Cache[T], error) {
	if err := validateCacheType[T](); err != nil {
		return nil, err
	}

	options := cacheOptions{
		codec:          JSONCodec{},
		isNotFound:     defaultCacheIsNotFound,
		negativeMarker: []byte{defaultCacheNegativeMarker},
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
		client:         client,
		prefix:         options.prefix,
		ttl:            options.ttl,
		jitter:         options.jitter,
		negativeTTL:    options.negativeTTL,
		negativeMarker: options.negativeMarker,
		codec:          options.codec,
		redisEncoded:   isRedisEncoded[T](),
		isNotFound:     options.isNotFound,
	}, nil
}

func validateCacheType[T any]() error {
	typ := reflect.TypeFor[T]()
	if typ.Kind() == reflect.Interface {
		return fmt.Errorf("%w: interface cache type %s is not supported", ErrInvalidCache, typ)
	}

	return nil
}

func validateCacheOptions(client *Client, opts cacheOptions) error {
	if client == nil || client.conn == nil {
		return ErrInvalidCache
	}

	if opts.ttl < 0 || opts.jitter < 0 || opts.negativeTTL < 0 {
		return ErrInvalidTTL
	}

	if len(opts.negativeMarker) == 0 {
		return ErrInvalidCacheMarker
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

// WithCacheNegativeMarker configures the value used to represent cached
// not-found results.
//
// The marker must not be empty. Its exact byte sequence is reserved and must
// not be stored as a regular cache value. The default marker is []byte{0}.
//
// Cache instances sharing the same keyspace must use the same negative marker.
// When changing the marker, delete existing negative entries or use a new
// cache prefix.
func WithCacheNegativeMarker(marker []byte) CacheOption {
	marker = bytes.Clone(marker)

	return func(opts *cacheOptions) {
		opts.negativeMarker = marker
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
		return c.load(ctx, key, loader)
	})

	select {
	case <-ctx.Done():
		return zero, ctx.Err()

	case result := <-ch:
		if result.Err != nil {
			return zero, result.Err
		}

		loaded, ok := result.Val.(T)
		if !ok {
			return zero, ErrInvalidCacheEntry
		}

		return loaded, nil
	}
}

// Set stores a typed value in cache using default TTL.
func (c *Cache[T]) Set(ctx context.Context, key string, value T) error {
	encoded, err := c.encode(value)
	if err != nil {
		return err
	}

	return c.client.conn.Set(ctx, c.key(key), encoded, c.expiration(c.ttl)).Err()
}

// Delete removes a value from cache.
func (c *Cache[T]) Delete(ctx context.Context, key string) error {
	return c.client.conn.Del(ctx, c.key(key)).Err()
}

// Forget removes an in-flight loader for the key from singleflight.
func (c *Cache[T]) Forget(key string) {
	c.group.Forget(c.key(key))
}

func (c *Cache[T]) load(
	ctx context.Context,
	key string,
	loader Loader[T],
) (T, error) {
	value, err := loader(ctx)
	if err == nil {
		if setErr := c.Set(ctx, key, value); setErr != nil {
			return value, setErr
		}

		return value, nil
	}

	if !c.isNotFound(err) {
		return value, err
	}

	notFoundErr := normalizeCacheNotFound(err)

	if c.negativeTTL > 0 {
		if setErr := c.setNegative(ctx, key); setErr != nil {
			return value, errors.Join(notFoundErr, setErr)
		}
	}

	return value, notFoundErr
}

func (c *Cache[T]) get(ctx context.Context, key string) (T, cacheState, error) {
	var zero T

	cmd := c.client.conn.Get(ctx, c.key(key))
	data, err := cmd.Bytes()
	if err != nil {
		if errors.Is(err, rdb.Nil) {
			return zero, cacheMiss, nil
		}

		return zero, cacheMiss, err
	}

	if bytes.Equal(data, c.negativeMarker) {
		return zero, cacheNegative, nil
	}

	value, err := c.decode(cmd, data)
	if err != nil {
		return zero, cacheMiss, err
	}

	return value, cacheHit, nil
}

func (c *Cache[T]) setNegative(ctx context.Context, key string) error {
	if c.negativeTTL <= 0 {
		return nil
	}

	return c.client.conn.Set(
		ctx,
		c.key(key),
		c.negativeMarker,
		c.expiration(c.negativeTTL),
	).Err()
}

func (c *Cache[T]) encode(value T) (any, error) {
	if c.redisEncoded {
		return value, nil
	}

	return c.codec.Marshal(value)
}

func (c *Cache[T]) decode(cmd *rdb.StringCmd, data []byte) (T, error) {
	if c.redisEncoded {
		return scanCacheValue[T](cmd)
	}

	return decodeCacheValue[T](c.codec, data)
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

func normalizeCacheNotFound(err error) error {
	if errors.Is(err, ErrKeyNotFound) {
		return err
	}

	return fmt.Errorf("%w: %w", ErrKeyNotFound, err)
}

func defaultCacheIsNotFound(err error) bool {
	return errors.Is(err, ErrKeyNotFound) || errors.Is(err, rdb.Nil)
}

func isRedisEncoded[T any]() bool {
	var value T

	switch any(value).(type) {
	case string, *string,
		[]byte,
		int, *int,
		int8, *int8,
		int16, *int16,
		int32, *int32,
		int64, *int64,
		uint, *uint,
		uint8, *uint8,
		uint16, *uint16,
		uint32, *uint32,
		uint64, *uint64,
		float32, *float32,
		float64, *float64,
		bool, *bool,
		time.Time, *time.Time,
		time.Duration, *time.Duration,
		net.IP:
		return true
	default:
		return false
	}
}

func decodeCacheInto[T any](decode func(dst any) error) (T, error) {
	var zero T

	typ := reflect.TypeFor[T]()

	if typ.Kind() != reflect.Pointer {
		var value T

		if err := decode(&value); err != nil {
			return zero, err
		}

		return value, nil
	}

	value := reflect.New(typ.Elem())

	if err := decode(value.Interface()); err != nil {
		return zero, err
	}

	// reflect.New returns PointerTo(typ.Elem()), which may differ
	// from T when T is a defined pointer type.
	if value.Type() != typ {
		if !value.CanConvert(typ) {
			return zero, ErrInvalidCacheEntry
		}

		value = value.Convert(typ)
	}

	decoded, ok := value.Interface().(T)
	if !ok {
		return zero, ErrInvalidCacheEntry
	}

	return decoded, nil
}

func scanCacheValue[T any](cmd *rdb.StringCmd) (T, error) {
	return decodeCacheInto[T](cmd.Scan)
}

func decodeCacheValue[T any](codec Codec, data []byte) (T, error) {
	return decodeCacheInto[T](func(dst any) error {
		return codec.Unmarshal(data, dst)
	})
}
