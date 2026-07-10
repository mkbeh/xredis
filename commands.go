package xredis

import (
	"context"
	"errors"
	"time"

	rdb "github.com/redis/go-redis/v9"
)

// Exists returns whether key exists.
func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	count, err := c.conn.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}

	return count == 1, nil
}

// HExists returns whether field is an existing field in the hash stored at key.
func (c *Client) HExists(ctx context.Context, key, field string) (bool, error) {
	return c.conn.HExists(ctx, key, field).Result()
}

// HIncrBy increments the number stored at field in the hash stored at key by increment.
func (c *Client) HIncrBy(ctx context.Context, key, field string, incr int64) error {
	return c.conn.HIncrBy(ctx, key, field, incr).Err()
}

// HGetAll returns all fields and values of the hash stored at key and scans the result into dst.
//
// It returns ok=false when the hash does not exist or has no fields.
func (c *Client) HGetAll(ctx context.Context, key string, dst any) (bool, error) {
	if dst == nil {
		return false, ErrInvalidHashObject
	}

	res := c.conn.HGetAll(ctx, key)
	if err := res.Err(); err != nil {
		return false, err
	}

	if len(res.Val()) == 0 {
		return false, nil
	}

	if err := res.Scan(dst); err != nil {
		return false, err
	}

	return true, nil
}

// HGet returns the value associated with field in the hash stored at key.
//
// It returns ok=false when the hash or field does not exist.
func (c *Client) HGet(ctx context.Context, key, field string) (string, bool, error) {
	value, err := c.conn.HGet(ctx, key, field).Result()
	if err != nil {
		if errors.Is(err, rdb.Nil) {
			return "", false, nil
		}

		return "", false, err
	}

	return value, true, nil
}

// HSet sets hash fields and optionally applies TTL to the hash key.
//
// values is passed to go-redis HSet, so it supports the same input formats:
// flat field-value pairs, slices, maps, structs, and pointers to structs.
//
// Examples:
//
//	HSet(ctx, "user:42", 0, "name", "Bob", "age", 30)
//	HSet(ctx, "user:42", time.Hour, map[string]any{"name": "Bob", "age": 30})
//	HSet(ctx, "user:42", time.Hour, UserHash{Name: "Bob"})
//
// Struct values are parsed by go-redis using redis tags.
//
// ttl < 0 returns ErrInvalidTTL.
// ttl == 0 leaves the hash expiration unchanged.
// ttl > 0 applies the expiration to the hash key after HSET.
func (c *Client) HSet(ctx context.Context, key string, ttl time.Duration, values ...any) error {
	if ttl < 0 {
		return ErrInvalidTTL
	}

	if len(values) == 0 {
		return ErrInvalidHashObject
	}

	if ttl == 0 {
		return c.conn.HSet(ctx, key, values...).Err()
	}

	pipe := c.conn.TxPipeline()
	pipe.HSet(ctx, key, values...)
	pipe.Expire(ctx, key, ttl)

	cmders, err := pipe.Exec(ctx)
	if err != nil {
		return err
	}

	for _, cmd := range cmders {
		if err = cmd.Err(); err != nil {
			return err
		}
	}

	return nil
}

// HDel deletes fields from the hash stored at key.
//
// It returns the number of fields that were removed.
func (c *Client) HDel(ctx context.Context, key string, fields ...string) (int64, error) {
	return c.conn.HDel(ctx, key, fields...).Result()
}

// Get reads a Redis string value and scans it into dst.
//
// It returns ok=false when the key does not exist.
func (c *Client) Get(ctx context.Context, key string, dst any) (bool, error) {
	if err := c.conn.Get(ctx, key).Scan(dst); err != nil {
		if errors.Is(err, rdb.Nil) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

// GetDel reads the value stored at key and atomically deletes the key.
//
// It returns ok=false when the key does not exist.
func (c *Client) GetDel(ctx context.Context, key string) (string, bool, error) {
	value, err := c.conn.GetDel(ctx, key).Result()
	if err != nil {
		if errors.Is(err, rdb.Nil) {
			return "", false, nil
		}

		return "", false, err
	}

	return value, true, nil
}

// GetEx reads the value stored at key and atomically updates its expiration.
//
// ttl < 0 returns ErrInvalidTTL.
// ttl == 0 removes the existing expiration.
// ttl > 0 applies the given expiration.
//
// It returns ok=false when the key does not exist.
func (c *Client) GetEx(
	ctx context.Context,
	key string,
	ttl time.Duration,
) (string, bool, error) {
	if ttl < 0 {
		return "", false, ErrInvalidTTL
	}

	value, err := c.conn.GetEx(ctx, key, ttl).Result()
	if err != nil {
		if errors.Is(err, rdb.Nil) {
			return "", false, nil
		}

		return "", false, err
	}

	return value, true, nil
}

// GetStruct reads an encoded Redis value and unmarshals it into dst.
//
// It returns ok=false when the key does not exist.
func (c *Client) GetStruct(ctx context.Context, key string, dst any) (bool, error) {
	data, err := c.conn.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, rdb.Nil) {
			return false, nil
		}

		return false, err
	}

	if err = c.codec.Unmarshal(data, dst); err != nil {
		return false, err
	}

	return true, nil
}

// GetStructDel reads an encoded value, atomically deletes the key,
// and decodes the value into dst.
//
// It returns ok=false when the key does not exist.
func (c *Client) GetStructDel(ctx context.Context, key string, dst any) (bool, error) {
	data, err := c.conn.GetDel(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, rdb.Nil) {
			return false, nil
		}

		return false, err
	}

	if err = c.codec.Unmarshal(data, dst); err != nil {
		return false, err
	}

	return true, nil
}

// GetStructEx reads an encoded value, atomically updates its expiration,
// and decodes the value into dst.
//
// ttl < 0 returns ErrInvalidTTL.
// ttl == 0 removes the existing expiration.
// ttl > 0 applies the given expiration.
//
// It returns ok=false when the key does not exist.
func (c *Client) GetStructEx(
	ctx context.Context,
	key string,
	dst any,
	ttl time.Duration,
) (bool, error) {
	if ttl < 0 {
		return false, ErrInvalidTTL
	}

	data, err := c.conn.GetEx(ctx, key, ttl).Bytes()
	if err != nil {
		if errors.Is(err, rdb.Nil) {
			return false, nil
		}

		return false, err
	}

	if err = c.codec.Unmarshal(data, dst); err != nil {
		return false, err
	}

	return true, nil
}

// Set executes Redis SET command.
func (c *Client) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	if ttl < 0 {
		return ErrInvalidTTL
	}

	return c.conn.Set(ctx, key, value, ttl).Err()
}

// SetNX sets key to value only when key does not exist.
//
// It returns ok=false when the key already exists.
func (c *Client) SetNX(ctx context.Context, key string, value any, ttl time.Duration) (bool, error) {
	if ttl < 0 {
		return false, ErrInvalidTTL
	}

	return c.conn.SetNX(ctx, key, value, ttl).Result()
}

// SetXX sets key to value only when key already exists.
//
// It returns ok=false when the key does not exist.
func (c *Client) SetXX(ctx context.Context, key string, value any, ttl time.Duration) (bool, error) {
	if ttl < 0 {
		return false, ErrInvalidTTL
	}

	return c.conn.SetXX(ctx, key, value, ttl).Result()
}

// SetStruct marshals value and stores it using Redis SET command.
func (c *Client) SetStruct(ctx context.Context, key string, value any, ttl time.Duration) error {
	if ttl < 0 {
		return ErrInvalidTTL
	}

	data, err := c.codec.Marshal(value)
	if err != nil {
		return err
	}

	return c.Set(ctx, key, data, ttl)
}

// SetStructNX marshals value and stores it only when key does not exist.
//
// It returns ok=false when the key already exists.
func (c *Client) SetStructNX(ctx context.Context, key string, value any, ttl time.Duration) (bool, error) {
	if ttl < 0 {
		return false, ErrInvalidTTL
	}

	data, err := c.codec.Marshal(value)
	if err != nil {
		return false, err
	}

	return c.SetNX(ctx, key, data, ttl)
}

// SetStructXX marshals value and stores it only when key already exists.
//
// It returns ok=false when the key does not exist.
func (c *Client) SetStructXX(ctx context.Context, key string, value any, ttl time.Duration) (bool, error) {
	if ttl < 0 {
		return false, ErrInvalidTTL
	}

	data, err := c.codec.Marshal(value)
	if err != nil {
		return false, err
	}

	return c.SetXX(ctx, key, data, ttl)
}

// Bool reads a Redis string value as bool.
func (c *Client) Bool(ctx context.Context, key string) (val, ok bool, err error) {
	res := c.conn.Get(ctx, key)
	val, err = res.Bool()
	if err != nil {
		if errors.Is(err, rdb.Nil) {
			return val, false, nil
		}

		return val, false, err
	}

	return val, true, nil
}

// Bytes reads a Redis string value as bytes.
func (c *Client) Bytes(ctx context.Context, key string) (val []byte, ok bool, err error) {
	res := c.conn.Get(ctx, key)
	val, err = res.Bytes()
	if err != nil {
		if errors.Is(err, rdb.Nil) {
			return val, false, nil
		}

		return val, false, err
	}

	return val, true, nil
}

// Float64 reads a Redis string value as float64.
func (c *Client) Float64(ctx context.Context, key string) (val float64, ok bool, err error) {
	res := c.conn.Get(ctx, key)
	val, err = res.Float64()
	if err != nil {
		if errors.Is(err, rdb.Nil) {
			return val, false, nil
		}

		return val, false, err
	}

	return val, true, nil
}

// Int reads a Redis string value as int.
func (c *Client) Int(ctx context.Context, key string) (val int, ok bool, err error) {
	res := c.conn.Get(ctx, key)
	val, err = res.Int()
	if err != nil {
		if errors.Is(err, rdb.Nil) {
			return val, false, nil
		}

		return val, false, err
	}

	return val, true, nil
}

// Int64 reads a Redis string value as int64.
func (c *Client) Int64(ctx context.Context, key string) (val int64, ok bool, err error) {
	res := c.conn.Get(ctx, key)
	val, err = res.Int64()
	if err != nil {
		if errors.Is(err, rdb.Nil) {
			return val, false, nil
		}

		return val, false, err
	}

	return val, true, nil
}

// Uint64 reads a Redis string value as uint64.
func (c *Client) Uint64(ctx context.Context, key string) (val uint64, ok bool, err error) {
	res := c.conn.Get(ctx, key)
	val, err = res.Uint64()
	if err != nil {
		if errors.Is(err, rdb.Nil) {
			return val, false, nil
		}

		return val, false, err
	}

	return val, true, nil
}

// String reads a Redis string value as string.
func (c *Client) String(ctx context.Context, key string) (val string, ok bool, err error) {
	res := c.conn.Get(ctx, key)
	val, err = res.Result()
	if err != nil {
		if errors.Is(err, rdb.Nil) {
			return val, false, nil
		}

		return val, false, err
	}

	return val, true, nil
}

// Incr increments the integer value stored at key.
func (c *Client) Incr(ctx context.Context, key string) error {
	return c.conn.Incr(ctx, key).Err()
}

// Decr decrements the integer value stored at key.
func (c *Client) Decr(ctx context.Context, key string) error {
	return c.conn.Decr(ctx, key).Err()
}

// Delete deletes key.
func (c *Client) Delete(ctx context.Context, key string) error {
	return c.conn.Del(ctx, key).Err()
}
