package xredis

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/google/uuid"
	rdb "github.com/redis/go-redis/v9"
)

const (
	versionedStoreValueField    = "value"
	versionedStoreRevisionField = "revision"
)

// versionedStoreCreateScript creates a versioned value only when the Redis key
// does not exist.
//
// KEYS[1] - versioned value key
// ARGV[1] - encoded value
// ARGV[2] - revision
// ARGV[3] - expiration in milliseconds, 0 means no expiration
var versionedStoreCreateScript = rdb.NewScript(`
if redis.call("EXISTS", KEYS[1]) == 1 then
	return 0
end

redis.call(
	"HSET",
	KEYS[1],
	"value",
	ARGV[1],
	"revision",
	ARGV[2]
)

local expiration = tonumber(ARGV[3])

if expiration > 0 then
	redis.call("PEXPIRE", KEYS[1], expiration)
end

return 1
`)

// versionedStoreCompareAndSwapScript swaps a versioned value only when the
// stored revision matches the expected revision.
//
// KEYS[1] - versioned value key
// ARGV[1] - expected revision
// ARGV[2] - new encoded value
// ARGV[3] - new revision
// ARGV[4] - expiration in milliseconds:
//   - -1 preserves the existing expiration
//   - 0 removes the existing expiration
//   - a positive value applies a new expiration
var versionedStoreCompareAndSwapScript = rdb.NewScript(`
local current_revision = redis.call(
	"HGET",
	KEYS[1],
	"revision"
)

if not current_revision or current_revision ~= ARGV[1] then
	return 0
end

redis.call(
	"HSET",
	KEYS[1],
	"value",
	ARGV[2],
	"revision",
	ARGV[3]
)

local expiration = tonumber(ARGV[4])

if expiration > 0 then
	redis.call("PEXPIRE", KEYS[1], expiration)
elseif expiration == 0 then
	redis.call("PERSIST", KEYS[1])
end

return 1
`)

// versionedStoreCompareAndDeleteScript deletes a versioned value only when the
// stored revision matches the expected revision.
//
// KEYS[1] - versioned value key
// ARGV[1] - expected revision
var versionedStoreCompareAndDeleteScript = rdb.NewScript(`
local current_revision = redis.call(
	"HGET",
	KEYS[1],
	"revision"
)

if not current_revision or current_revision ~= ARGV[1] then
	return 0
end

return redis.call("DEL", KEYS[1])
`)

// Revision is an opaque optimistic-concurrency token.
//
// A new revision is generated whenever a versioned value is created or
// successfully updated.
type Revision string

// VersionedValue contains a decoded value and its current revision.
type VersionedValue[T any] struct {
	// Value is the decoded stored value.
	Value T

	// Revision identifies the current version of Value.
	Revision Revision
}

// VersionedStore stores typed values with optimistic-concurrency revisions.
//
// Each value is stored as a Redis hash containing an encoded value and an
// opaque revision. Values must be read and modified through the same
// VersionedStore representation.
//
// VersionedStore is safe for concurrent use when its configured Codec is safe
// for concurrent use.
type VersionedStore[T any] struct {
	client *Client
	prefix string
	codec  Codec
}

// VersionedStoreOption configures VersionedStore.
type VersionedStoreOption func(*versionedStoreOptions)

type versionedStoreOptions struct {
	prefix string
	codec  Codec
}

// NewVersionedStore creates a typed versioned value store.
//
// The client Codec is used by default. If the client does not provide one,
// JSONCodec is used.
func NewVersionedStore[T any](
	client *Client,
	opts ...VersionedStoreOption,
) (*VersionedStore[T], error) {
	if err := validateVersionedStoreType[T](); err != nil {
		return nil, err
	}

	options := versionedStoreOptions{
		codec: JSONCodec{},
	}

	if client != nil && client.codec != nil {
		options.codec = client.codec
	}

	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}

	if client == nil || client.conn == nil || options.codec == nil {
		return nil, ErrInvalidVersionedStore
	}

	return &VersionedStore[T]{
		client: client,
		prefix: options.prefix,
		codec:  options.codec,
	}, nil
}

// WithVersionedStorePrefix configures the Redis key prefix.
func WithVersionedStorePrefix(prefix string) VersionedStoreOption {
	return func(opts *versionedStoreOptions) {
		opts.prefix = prefix
	}
}

// WithVersionedStoreCodec configures the value codec.
//
// The codec may be used concurrently by multiple store operations.
func WithVersionedStoreCodec(codec Codec) VersionedStoreOption {
	return func(opts *versionedStoreOptions) {
		if codec != nil {
			opts.codec = codec
		}
	}
}

// Get reads a value together with its current revision.
//
// It returns ok=false when the Redis key does not exist.
func (s *VersionedStore[T]) Get(
	ctx context.Context,
	key string,
) (VersionedValue[T], bool, error) {
	var zero VersionedValue[T]

	if err := s.validateKey(key); err != nil {
		return zero, false, err
	}

	result, err := s.client.conn.HMGet(
		ctx,
		s.key(key),
		versionedStoreValueField,
		versionedStoreRevisionField,
	).Result()
	if err != nil {
		return zero, false, err
	}

	if len(result) != 2 {
		return zero, false, ErrInvalidEntry
	}

	valueRaw, revisionRaw := result[0], result[1]

	if valueRaw == nil && revisionRaw == nil {
		return zero, false, nil
	}

	if valueRaw == nil || revisionRaw == nil {
		return zero, false, ErrInvalidEntry
	}

	data, ok := versionedStoreBytes(valueRaw)
	if !ok {
		return zero, false, ErrInvalidEntry
	}

	revision, ok := versionedStoreRevision(revisionRaw)
	if !ok {
		return zero, false, ErrInvalidEntry
	}

	value, err := decodeVersionedStoreValue[T](
		s.codec,
		data,
	)
	if err != nil {
		return zero, false, err
	}

	return VersionedValue[T]{
		Value:    value,
		Revision: revision,
	}, true, nil
}

// SetIfAbsent stores a value only when the Redis key does not exist.
//
// It returns created=false when the key already exists.
//
// expiration == 0 stores the value without expiration.
// expiration > 0 stores the value with the given expiration.
// Negative expiration values, including KeepTTL, return ErrInvalidTTL.
func (s *VersionedStore[T]) SetIfAbsent(
	ctx context.Context,
	key string,
	value T,
	expiration time.Duration,
) (revision Revision, created bool, err error) {
	if err = s.validateKey(key); err != nil {
		return "", false, err
	}

	if err := validateCreateExpiration(expiration); err != nil {
		return "", false, err
	}

	data, err := s.codec.Marshal(value)
	if err != nil {
		return "", false, err
	}

	revision = Revision(uuid.NewString())

	created, err = s.setIfAbsent(
		ctx,
		key,
		data,
		revision,
		expiration,
	)
	if err != nil {
		return "", false, err
	}

	if !created {
		return "", false, nil
	}

	return revision, true, nil
}

// CompareAndSwap replaces a value only when its current revision matches
// expectedRevision.
//
// A successful update returns a new revision. It returns swapped=false when
// the key does not exist or its revision has changed.
//
// expiration == KeepTTL preserves the existing expiration.
// expiration == 0 removes the existing expiration.
// expiration > 0 applies the given expiration.
// Any other negative value returns ErrInvalidTTL.
func (s *VersionedStore[T]) CompareAndSwap(
	ctx context.Context,
	key string,
	expectedRevision Revision,
	value T,
	expiration time.Duration,
) (revision Revision, swapped bool, err error) {
	if err = s.validateRevision(key, expectedRevision); err != nil {
		return "", false, err
	}

	if err = validateUpdateExpiration(expiration); err != nil {
		return "", false, err
	}

	data, err := s.codec.Marshal(value)
	if err != nil {
		return "", false, err
	}

	revision = Revision(uuid.NewString())

	result, err := versionedStoreCompareAndSwapScript.Run(
		ctx,
		s.client.conn,
		[]string{s.key(key)},
		string(expectedRevision),
		data,
		string(revision),
		expirationToMs(expiration),
	).Int64()
	if err != nil {
		return "", false, err
	}

	if result != 1 {
		return "", false, nil
	}

	return revision, true, nil
}

// CompareAndDelete deletes a value only when its current revision matches
// expectedRevision.
//
// It returns deleted=false when the key does not exist or its revision has
// changed.
func (s *VersionedStore[T]) CompareAndDelete(
	ctx context.Context,
	key string,
	expectedRevision Revision,
) (deleted bool, err error) {
	if err = s.validateRevision(key, expectedRevision); err != nil {
		return false, err
	}

	result, err := versionedStoreCompareAndDeleteScript.Run(
		ctx,
		s.client.conn,
		[]string{s.key(key)},
		string(expectedRevision),
	).Int64()
	if err != nil {
		return false, err
	}

	return result == 1, nil
}

func (s *VersionedStore[T]) setIfAbsent(
	ctx context.Context,
	key string,
	data []byte,
	revision Revision,
	expiration time.Duration,
) (bool, error) {
	created, err := versionedStoreCreateScript.Run(
		ctx,
		s.client.conn,
		[]string{s.key(key)},
		data,
		string(revision),
		durationToMs(expiration),
	).Int64()
	if err != nil {
		return false, err
	}

	return created == 1, nil
}

func (s *VersionedStore[T]) validateKey(key string) error {
	if s == nil ||
		s.client == nil ||
		s.client.conn == nil ||
		s.codec == nil ||
		key == "" {
		return ErrInvalidVersionedStore
	}

	return nil
}

func (s *VersionedStore[T]) validateRevision(
	key string,
	revision Revision,
) error {
	if err := s.validateKey(key); err != nil {
		return err
	}

	if revision == "" {
		return ErrInvalidVersionedStore
	}

	return nil
}

func (s *VersionedStore[T]) key(key string) string {
	return s.prefix + key
}

func validateVersionedStoreType[T any]() error {
	typ := reflect.TypeFor[T]()

	if typ.Kind() == reflect.Interface {
		return fmt.Errorf(
			"%w: interface value type %s is not supported",
			ErrInvalidVersionedStore,
			typ,
		)
	}

	return nil
}

func versionedStoreBytes(value any) ([]byte, bool) {
	switch value := value.(type) {
	case string:
		return []byte(value), true

	case []byte:
		return value, true

	default:
		return nil, false
	}
}

func versionedStoreRevision(value any) (Revision, bool) {
	switch value := value.(type) {
	case string:
		if value == "" {
			return "", false
		}

		return Revision(value), true

	case []byte:
		if len(value) == 0 {
			return "", false
		}

		return Revision(value), true

	default:
		return "", false
	}
}

func decodeVersionedStoreValue[T any](
	codec Codec,
	data []byte,
) (T, error) {
	return decodeInto[T](func(dst any) error {
		return codec.Unmarshal(data, dst)
	})
}
