package xredis

import (
	"context"
	"time"

	"github.com/google/uuid"
	rdb "github.com/redis/go-redis/v9"
)

// lockAcquireFencedScript atomically acquires a lock and issues a fencing token.
//
// The fencing counter is incremented only when the lock is acquired.
// Counter TTL is optional and is used only as retention for bounded-lifecycle
// resources.
//
// KEYS[1] - lock key
// KEYS[2] - fencing counter key
// ARGV[1] - owner token
// ARGV[2] - lock TTL in milliseconds
// ARGV[3] - fencing counter TTL in milliseconds, 0 means no expiration
var lockAcquireFencedScript = rdb.NewScript(`
if redis.call("EXISTS", KEYS[1]) == 1 then
	return {0, 0}
end

local fencing_token = redis.call("INCR", KEYS[2])
local lock_ttl = tonumber(ARGV[2])
local counter_ttl = tonumber(ARGV[3])

redis.call("SET", KEYS[1], ARGV[1], "PX", lock_ttl)

if counter_ttl > 0 then
	redis.call("PEXPIRE", KEYS[2], counter_ttl)
end

return {1, fencing_token}
`)

// lockExtendFencedScript atomically extends a fenced lock and refreshes the
// fencing counter retention TTL when configured.
//
// KEYS[1] - lock key
// KEYS[2] - fencing counter key
// ARGV[1] - owner token
// ARGV[2] - lock TTL in milliseconds
// ARGV[3] - fencing counter TTL in milliseconds, 0 means no expiration
var lockExtendFencedScript = rdb.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
	local lock_ttl = tonumber(ARGV[2])
	local counter_ttl = tonumber(ARGV[3])

	local extended = redis.call("PEXPIRE", KEYS[1], lock_ttl)

	if extended == 1 and counter_ttl > 0 then
		redis.call("PEXPIRE", KEYS[2], counter_ttl)
	end

	return extended
end

return 0
`)

// FencedLock represents an acquired Redis lease lock with a fencing token.
//
// The fencing token is a monotonically increasing number. It is useful only if
// the protected resource rejects operations with stale fencing tokens.
type FencedLock struct {
	lock *Lock

	fencingKey   string
	fencingToken int64
	counterTTL   time.Duration
}

// FencedLockOption configures fenced lock behavior.
type FencedLockOption func(*fencedLockOptions)

type fencedLockOptions struct {
	counterTTL time.Duration
}

func newFencedLockOptions(opts ...FencedLockOption) fencedLockOptions {
	var o fencedLockOptions
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}
	return o
}

func validateFencedLockTTL(lockTTL, counterTTL time.Duration) error {
	if lockTTL <= 0 {
		return ErrInvalidTTL
	}

	if counterTTL < 0 {
		return ErrInvalidTTL
	}

	if counterTTL > 0 && counterTTL <= lockTTL {
		return ErrInvalidTTL
	}

	return nil
}

// WithFencingCounterTTL configures retention TTL for the fencing counter key.
//
// By default, the fencing counter has no expiration because fencing tokens
// should not reset. When this option is set, the counter TTL is refreshed on
// every successful fenced lock acquisition and extension.
//
// Use this option only when the protected resource has a bounded lifecycle and
// the retention window is safe for your application.
func WithFencingCounterTTL(ttl time.Duration) FencedLockOption {
	return func(opts *fencedLockOptions) {
		opts.counterTTL = ttl
	}
}

// Key returns the Redis lock key.
func (l *FencedLock) Key() string {
	if l == nil || l.lock == nil {
		return ""
	}

	return l.lock.Key()
}

// Token returns the lock owner token.
func (l *FencedLock) Token() string {
	if l == nil || l.lock == nil {
		return ""
	}

	return l.lock.Token()
}

// FencingKey returns the Redis fencing counter key.
func (l *FencedLock) FencingKey() string {
	if l == nil {
		return ""
	}

	return l.fencingKey
}

// FencingToken returns the monotonically increasing fencing token.
func (l *FencedLock) FencingToken() int64 {
	if l == nil {
		return 0
	}

	return l.fencingToken
}

// TryFencedLock tries to acquire a Redis lock with a fencing token.
//
// ttl controls the lock lease duration and is applied only to key.
// fencingKey is used as a monotonically increasing Redis counter.
//
// By default, fencingKey has no expiration because fencing tokens must not reset.
// Use WithFencingCounterTTL only when the protected resource has a bounded lifecycle.
//
// For Redis Cluster, key and fencingKey must belong to the same hash slot
// and therefore be handled by the same cluster master node. Use hash tags to
// guarantee this, for example:
//
//	key:        lock:{order:42}
//	fencingKey: fence:{order:42}
func (c *Client) TryFencedLock(
	ctx context.Context,
	key string,
	fencingKey string,
	ttl time.Duration,
	opts ...FencedLockOption,
) (*FencedLock, bool, error) {
	return c.TryFencedLockWithToken(ctx, key, fencingKey, uuid.NewString(), ttl, opts...)
}

// TryFencedLockWithToken tries to acquire a Redis lock using the provided owner
// token and returns a fencing token.
//
// Token must be unique per lock attempt. Reusing tokens across independent lock
// attempts may make ownership checks unsafe.
//
// ttl controls the lock lease duration and is applied only to key.
// fencingKey is used as a monotonically increasing Redis counter.
//
// By default, fencingKey has no expiration because fencing tokens must not reset.
// Use WithFencingCounterTTL only when the protected resource has a bounded lifecycle.
//
// For Redis Cluster, key and fencingKey must belong to the same hash slot
// and therefore be handled by the same cluster master node. Use hash tags to
// guarantee this, for example:
//
//	key:        lock:{order:42}
//	fencingKey: fence:{order:42}
func (c *Client) TryFencedLockWithToken(
	ctx context.Context,
	key string,
	fencingKey string,
	token string,
	ttl time.Duration,
	opts ...FencedLockOption,
) (*FencedLock, bool, error) {
	if c == nil || c.conn == nil || key == "" || fencingKey == "" || token == "" {
		return nil, false, ErrInvalidLock
	}

	options := newFencedLockOptions(opts...)

	if err := validateFencedLockTTL(ttl, options.counterTTL); err != nil {
		return nil, false, err
	}

	result, err := lockAcquireFencedScript.Run(
		ctx,
		c.conn,
		[]string{key, fencingKey},
		token,
		ttlToMs(ttl),
		ttlToMs(options.counterTTL),
	).Slice()
	if err != nil {
		return nil, false, err
	}

	acquired, fencingToken, err := parseFencedLockResult(result)
	if err != nil {
		return nil, false, err
	}

	if !acquired {
		return nil, false, nil
	}

	return &FencedLock{
		lock: &Lock{
			client: c,
			key:    key,
			token:  token,
		},
		fencingKey:   fencingKey,
		fencingToken: fencingToken,
		counterTTL:   options.counterTTL,
	}, true, nil
}

// Unlock releases the lock if it is still owned by this FencedLock.
func (l *FencedLock) Unlock(ctx context.Context) error {
	if l == nil || l.lock == nil {
		return ErrInvalidLock
	}

	return l.lock.Unlock(ctx)
}

// Extend extends the lock TTL if it is still owned by this FencedLock.
//
// If fencing counter TTL is configured, it is refreshed after a successful lock
// extension.
func (l *FencedLock) Extend(ctx context.Context, ttl time.Duration) (bool, error) {
	if l == nil || l.lock == nil || l.fencingKey == "" {
		return false, ErrInvalidLock
	}

	if err := l.lock.validate(); err != nil {
		return false, err
	}

	if err := validateFencedLockTTL(ttl, l.counterTTL); err != nil {
		return false, err
	}

	extended, err := lockExtendFencedScript.Run(
		ctx,
		l.lock.client.conn,
		[]string{l.lock.key, l.fencingKey},
		l.lock.token,
		ttlToMs(ttl),
		ttlToMs(l.counterTTL),
	).Int64()
	if err != nil {
		return false, err
	}

	return extended == 1, nil
}

func parseFencedLockResult(result []any) (bool, int64, error) {
	if len(result) != 2 {
		return false, 0, ErrInvalidLock
	}

	acquired, ok := result[0].(int64)
	if !ok {
		return false, 0, ErrInvalidLock
	}

	if acquired == 0 {
		return false, 0, nil
	}

	fencingToken, ok := result[1].(int64)
	if !ok || fencingToken <= 0 {
		return false, 0, ErrInvalidLock
	}

	return true, fencingToken, nil
}
