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
// ARGV[3] - value TTL in milliseconds, 0 means no expiration
var compareAndSwapScript = rdb.NewScript(`
local current = redis.call("GET", KEYS[1])
if not current or current ~= ARGV[1] then
	return 0
end

local ttl = tonumber(ARGV[3])

if ttl > 0 then
	redis.call("SET", KEYS[1], ARGV[2], "PX", ttl)
else
	redis.call("SET", KEYS[1], ARGV[2])
end

return 1
`)

// CompareAndDelete deletes key only when its current Redis string value equals
// the expected old value.
//
// expected is passed as a raw Redis argument, so this method is compatible with
// values stored through Set, not SetStruct.
//
// Use CompareAndDeleteStruct for values stored through SetStruct.
//
// It returns deleted=false when the key does not exist or contains a different value.
func (c *Client) CompareAndDelete(ctx context.Context, key string, expected any) (deleted bool, err error) {
	return c.compareAndDelete(ctx, key, expected)
}

// CompareAndDeleteStruct deletes key only when its current encoded Redis string
// value equals the encoded expected old value.
//
// Values are encoded with the client Codec, so this method is compatible with
// SetStruct/GetStruct, not with raw Set/Get values.
//
// It returns deleted=false when the key does not exist or contains a different value.
func (c *Client) CompareAndDeleteStruct(
	ctx context.Context,
	key string,
	expected any,
) (deleted bool, err error) {
	expectedData, err := c.codec.Marshal(expected)
	if err != nil {
		return false, err
	}

	return c.compareAndDelete(ctx, key, expectedData)
}

// CompareAndSwap swaps value only when the current Redis string value equals
// the expected old value.
//
// expected and value are passed as raw Redis arguments, so this method is
// compatible with values stored through Set, not SetStruct.
//
// Use CompareAndSwapStruct for values stored through SetStruct.
//
// It returns swapped=false when the key does not exist or contains a different value.
// expiration == 0 stores the new value without TTL.
// expiration > 0 stores the new value with the given TTL.
// expiration < 0 returns ErrInvalidTTL.
func (c *Client) CompareAndSwap(
	ctx context.Context,
	key string,
	expected any,
	value any,
	expiration time.Duration,
) (swapped bool, err error) {
	return c.compareAndSwap(ctx, key, expected, value, expiration)
}

// CompareAndSwapStruct swaps value only when the current encoded Redis string value
// equals the encoded expected old value.
//
// Values are encoded with the client Codec, so this method is compatible with
// SetStruct/GetStruct, not with raw Set/Get values.
//
// It returns swapped=false when the key does not exist or contains a different value.
// expiration == 0 stores the new value without TTL.
// expiration > 0 stores the new value with the given TTL.
// expiration < 0 returns ErrInvalidTTL.
func (c *Client) CompareAndSwapStruct(
	ctx context.Context,
	key string,
	expected any,
	value any,
	expiration time.Duration,
) (swapped bool, err error) {
	expectedData, err := c.codec.Marshal(expected)
	if err != nil {
		return false, err
	}

	valueData, err := c.codec.Marshal(value)
	if err != nil {
		return false, err
	}

	return c.compareAndSwap(ctx, key, expectedData, valueData, expiration)
}

func (c *Client) compareAndDelete(ctx context.Context, key string, expected any) (deleted bool, err error) {
	result, err := compareAndDeleteScript.Run(ctx, c.conn, []string{key}, expected).Int64()
	if err != nil {
		return false, err
	}

	return result == 1, nil
}

func (c *Client) compareAndSwap(
	ctx context.Context,
	key string,
	expected any,
	value any,
	expiration time.Duration,
) (swapped bool, err error) {
	if expiration < 0 {
		return false, ErrInvalidTTL
	}

	result, err := compareAndSwapScript.Run(
		ctx,
		c.conn,
		[]string{key},
		expected,
		value,
		durationToMs(expiration),
	).Int64()
	if err != nil {
		return false, err
	}

	return result == 1, nil
}
