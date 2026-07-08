package xredis

import (
	"context"
	"time"

	"github.com/google/uuid"
	rdb "github.com/redis/go-redis/v9"
)

// lockUnlockScript atomically releases a lock only if the stored owner token
// matches the caller token.
//
// KEYS[1] - lock key
// ARGV[1] - owner token
var lockUnlockScript = rdb.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("DEL", KEYS[1])
end

return 0
`)

// lockExtendScript atomically extends a lock only if the stored owner token
// matches the caller token.
//
// KEYS[1] - lock key
// ARGV[1] - owner token
// ARGV[2] - lock TTL in milliseconds
var lockExtendScript = rdb.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("PEXPIRE", KEYS[1], tonumber(ARGV[2]))
end

return 0
`)

// Lock represents an acquired Redis lease lock.
//
// Lock is token-based: Unlock and Extend only succeed while Redis still stores
// the same owner token.
type Lock struct {
	client *Client

	key   string
	token string
}

// Key returns the Redis lock key.
func (l *Lock) Key() string {
	if l == nil {
		return ""
	}

	return l.key
}

// Token returns the lock owner token.
func (l *Lock) Token() string {
	if l == nil {
		return ""
	}

	return l.token
}

// TryLock tries to acquire a Redis lock with ttl.
//
// It returns acquired=false when the lock already exists.
func (c *Client) TryLock(ctx context.Context, key string, ttl time.Duration) (*Lock, bool, error) {
	return c.TryLockWithToken(ctx, key, uuid.NewString(), ttl)
}

// TryLockWithToken tries to acquire a Redis lock using the provided owner token.
//
// Token must be unique per lock attempt. Reusing tokens across independent lock
// attempts may make ownership checks unsafe.
func (c *Client) TryLockWithToken(ctx context.Context, key, token string, ttl time.Duration) (*Lock, bool, error) {
	if c == nil || c.conn == nil || key == "" || token == "" {
		return nil, false, ErrInvalidLock
	}

	if ttl <= 0 {
		return nil, false, ErrInvalidTTL
	}

	acquired, err := c.conn.SetNX(ctx, key, token, ttl).Result()
	if err != nil {
		return nil, false, err
	}

	if !acquired {
		return nil, false, nil
	}

	return &Lock{
		client: c,
		key:    key,
		token:  token,
	}, true, nil
}

// Unlock releases the lock if it is still owned by this Lock.
//
// It returns ErrLockNotOwned if the lock expired, was deleted, or is owned by
// another token.
func (l *Lock) Unlock(ctx context.Context) error {
	if err := l.validate(); err != nil {
		return err
	}

	deleted, err := lockUnlockScript.Run(ctx, l.client.conn, []string{l.key}, l.token).Int64()
	if err != nil {
		return err
	}

	if deleted == 0 {
		return ErrLockNotOwned
	}

	return nil
}

// Extend extends the lock TTL if it is still owned by this Lock.
//
// It returns false when the lock expired, was deleted, or is owned by another
// token.
func (l *Lock) Extend(ctx context.Context, ttl time.Duration) (bool, error) {
	if err := l.validate(); err != nil {
		return false, err
	}

	if ttl <= 0 {
		return false, ErrInvalidTTL
	}

	extended, err := lockExtendScript.Run(ctx, l.client.conn, []string{l.key}, l.token, ttlToMs(ttl)).Int64()
	if err != nil {
		return false, err
	}

	return extended == 1, nil
}

func (l *Lock) validate() error {
	if l == nil || l.client == nil || l.client.conn == nil || l.key == "" || l.token == "" {
		return ErrInvalidLock
	}

	return nil
}

func ttlToMs(ttl time.Duration) int64 {
	ms := ttl / time.Millisecond
	if ttl%time.Millisecond != 0 {
		ms++
	}

	return int64(ms)
}
