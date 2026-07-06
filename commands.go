package xredis

import (
	"context"
	"errors"
	"reflect"
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
func (c *Client) HGetAll(ctx context.Context, key string, dst any) error {
	res := c.conn.HGetAll(ctx, key)
	if err := res.Err(); err != nil {
		return err
	}

	if len(res.Val()) == 0 {
		return ErrKeyNotFound
	}

	return res.Scan(dst)
}

// HGet returns the value associated with field in the hash stored at key and scans the result into dst.
func (c *Client) HGet(ctx context.Context, key, field string, dst any) error {
	if err := c.conn.HGet(ctx, key, field).Scan(dst); err != nil {
		if errors.Is(err, rdb.Nil) {
			return ErrKeyNotFound
		}

		return err
	}

	return nil
}

// HSet sets field in the hash stored at key to value and optionally applies TTL to the hash key.
func (c *Client) HSet(ctx context.Context, key, field string, value any, ttl time.Duration) error {
	if ttl < 0 {
		return ErrInvalidTTL
	}

	if err := validateRedisScalar(value); err != nil {
		return err
	}

	pipe := c.conn.TxPipeline()
	pipe.HSet(ctx, key, field, value)

	if ttl > 0 {
		pipe.Expire(ctx, key, ttl)
	}

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

// HSetObject sets redis-tagged fields of data object in the hash stored at key.
func (c *Client) HSetObject(ctx context.Context, key string, data any, ttl time.Duration) error {
	if ttl < 0 {
		return ErrInvalidTTL
	}

	value, err := structValue(data)
	if err != nil {
		return err
	}

	pipe := c.conn.TxPipeline()
	fields := 0

	typ := value.Type()
	for i := 0; i < value.NumField(); i++ {
		fieldInfo := typ.Field(i)
		tagValue, ok := fieldInfo.Tag.Lookup("redis")
		if !ok || tagValue == "-" {
			continue
		}

		field := value.Field(i)
		if !field.CanInterface() {
			continue
		}

		if err = validateRedisScalar(field.Interface()); err != nil {
			return err
		}

		pipe.HSet(ctx, key, tagValue, field.Interface())
		fields++
	}

	if ttl > 0 {
		pipe.Expire(ctx, key, ttl)
	}

	if fields == 0 && ttl == 0 {
		return nil
	}

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

// Get reads a Redis string value and scans it into dst.
func (c *Client) Get(ctx context.Context, key string, dst any) error {
	if err := c.conn.Get(ctx, key).Scan(dst); err != nil {
		if errors.Is(err, rdb.Nil) {
			return ErrKeyNotFound
		}

		return err
	}

	return nil
}

// Set executes Redis SET command.
func (c *Client) Set(ctx context.Context, key string, val any, expiration time.Duration) error {
	return c.conn.Set(ctx, key, val, expiration).Err()
}

// SetStruct marshals val and stores it using Redis SET command.
func (c *Client) SetStruct(ctx context.Context, key string, val any, expiration time.Duration) error {
	data, err := c.marshaller(val)
	if err != nil {
		return err
	}

	return c.conn.Set(ctx, key, data, expiration).Err()
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

// MassDelete deletes keys using pipeline.
func (c *Client) MassDelete(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return nil
	}

	cmders, err := c.conn.Pipelined(ctx, func(pipe rdb.Pipeliner) error {
		pipe.Del(ctx, keys...)
		return nil
	})
	if err != nil {
		return err
	}

	for _, cmder := range cmders {
		if err = cmder.Err(); err != nil {
			return err
		}
	}

	return nil
}

func validateRedisScalar(value any) error {
	v := reflect.ValueOf(value)
	if !v.IsValid() {
		return ErrInvalidFieldType
	}

	switch v.Kind() {
	case reflect.Pointer,
		reflect.Array,
		reflect.Map,
		reflect.Struct,
		reflect.Slice,
		reflect.Func,
		reflect.Chan,
		reflect.UnsafePointer:
		return ErrInvalidFieldType
	default:
		return nil
	}
}

func structValue(value any) (reflect.Value, error) {
	v := reflect.ValueOf(value)
	if !v.IsValid() {
		return reflect.Value{}, ErrInvalidFieldType
	}

	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return reflect.Value{}, ErrInvalidFieldType
		}
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return reflect.Value{}, ErrInvalidFieldType
	}

	return v, nil
}
