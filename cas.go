package xredis

import (
	"context"
	"time"

	rdb "github.com/redis/go-redis/v9"
)

// compareAndDeleteScript atomically deletes a key only if its current value
// matches the expected old value.
//
// KEYS[1] - key
// ARGV[1] - expected old value
var compareAndDeleteScript = rdb.NewScript(`
local current = redis.call("GET", KEYS[1])
if not current or current ~= ARGV[1] then
	return 0
end

return redis.call("DEL", KEYS[1])
`)

// compareAndSwapScript atomically swaps a value only if the current value
// matches the expected old value.
//
// KEYS[1] - key
// ARGV[1] - expected old value
// ARGV[2] - new value
// ARGV[3] - expiration in milliseconds:
//   - -1 preserves the existing expiration
//   - 0 removes the existing expiration
//   - a positive value applies a new expiration
var compareAndSwapScript = rdb.NewScript(`
local current = redis.call("GET", KEYS[1])
if not current or current ~= ARGV[1] then
	return 0
end

local expiration = tonumber(ARGV[3])

if expiration > 0 then
	redis.call("SET", KEYS[1], ARGV[2], "PX", expiration)
elseif expiration == 0 then
	redis.call("SET", KEYS[1], ARGV[2])
else
	redis.call("SET", KEYS[1], ARGV[2], "KEEPTTL")
end

return 1
`)

// hashCompareAndDeleteScript atomically deletes a hash field only if its
// current value matches the expected old value.
//
// KEYS[1] - hash key
// ARGV[1] - hash field
// ARGV[2] - expected old value
var hashCompareAndDeleteScript = rdb.NewScript(`
local current = redis.call("HGET", KEYS[1], ARGV[1])
if not current or current ~= ARGV[2] then
	return 0
end

return redis.call("HDEL", KEYS[1], ARGV[1])
`)

// hashCompareAndSwapScript atomically swaps a hash field only if its current
// value matches the expected old value.
//
// KEYS[1] - hash key
// ARGV[1] - hash field
// ARGV[2] - expected old value
// ARGV[3] - new value
var hashCompareAndSwapScript = rdb.NewScript(`
local current = redis.call("HGET", KEYS[1], ARGV[1])
if not current or current ~= ARGV[2] then
	return 0
end

redis.call("HSET", KEYS[1], ARGV[1], ARGV[3])

return 1
`)

// CompareAndDelete deletes key only when its current Redis string value equals
// the expected old value.
//
// expected is passed as a raw Redis argument and comparison is byte-for-byte.
//
// It returns deleted=false when the key does not exist or contains a different value.
func (c *Client) CompareAndDelete(ctx context.Context, key string, expected any) (deleted bool, err error) {
	result, err := compareAndDeleteScript.Run(ctx, c.conn, []string{key}, expected).Int64()
	if err != nil {
		return false, err
	}

	return result == 1, nil
}

// CompareAndSwap swaps value only when the current Redis string value equals
// the expected old value.
//
// expected and value are passed as raw Redis arguments and comparison is
// byte-for-byte.
//
// It returns swapped=false when the key does not exist or contains a different value.
// expiration == KeepTTL preserves the existing expiration.
// expiration == 0 stores the new value without expiration.
// expiration > 0 stores the new value with the given expiration.
// Any other negative value returns ErrInvalidTTL.
func (c *Client) CompareAndSwap(
	ctx context.Context,
	key string,
	expected any,
	value any,
	expiration time.Duration,
) (swapped bool, err error) {
	if err := validateUpdateExpiration(expiration); err != nil {
		return false, err
	}

	result, err := compareAndSwapScript.Run(
		ctx,
		c.conn,
		[]string{key},
		expected,
		value,
		expirationToMs(expiration),
	).Int64()
	if err != nil {
		return false, err
	}

	return result == 1, nil
}

// HCompareAndDelete deletes a hash field only when its current value equals
// the expected old value.
//
// expected is passed as a raw Redis argument and must use the same encoding as
// the value stored in the hash field.
//
// It returns deleted=false when the hash or field does not exist, or when the
// field contains a different value.
//
// If the deleted field is the last field in the hash, Redis removes the hash key.
func (c *Client) HCompareAndDelete(
	ctx context.Context,
	key string,
	field string,
	expected any,
) (deleted bool, err error) {
	result, err := hashCompareAndDeleteScript.Run(
		ctx,
		c.conn,
		[]string{key},
		field,
		expected,
	).Int64()
	if err != nil {
		return false, err
	}

	return result == 1, nil
}

// HCompareAndSwap swaps a hash field only when its current value equals the
// expected old value.
//
// expected and value are passed as raw Redis arguments and must use the same
// encoding as values stored in the hash field.
//
// It returns swapped=false when the hash or field does not exist, or when the
// field contains a different value.
//
// A successful swap preserves the existing expiration of the hash key.
func (c *Client) HCompareAndSwap(
	ctx context.Context,
	key string,
	field string,
	expected any,
	value any,
) (swapped bool, err error) {
	result, err := hashCompareAndSwapScript.Run(
		ctx,
		c.conn,
		[]string{key},
		field,
		expected,
		value,
	).Int64()
	if err != nil {
		return false, err
	}

	return result == 1, nil
}
