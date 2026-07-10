package xredis

import (
	"context"
	"time"

	rdb "github.com/redis/go-redis/v9"
)

// SetItem describes one Redis SET operation.
type SetItem struct {
	// Key is the Redis key.
	Key string

	// Value is passed directly to Redis by SetMany or encoded with the client
	// Codec by SetStructMany.
	Value any

	// Expiration is the key expiration.
	//
	// Zero means no expiration.
	Expiration time.Duration
}

// SetMany stores multiple raw Redis values using SET commands in one pipeline.
//
// Values are passed directly to Redis without Codec encoding.
//
// Values are written as independent SET commands, so this helper is safe to use
// with standalone Redis, Redis Cluster, and Ring clients.
//
// For very large input, split items into batches at the call site.
func (c *Client) SetMany(ctx context.Context, items []SetItem) error {
	if err := validatePipelineClient(c); err != nil {
		return err
	}

	if len(items) == 0 {
		return nil
	}

	_, err := c.conn.Pipelined(ctx, func(pipe rdb.Pipeliner) error {
		for _, item := range items {
			if item.Expiration < 0 {
				return ErrInvalidTTL
			}

			pipe.Set(ctx, item.Key, item.Value, item.Expiration)
		}

		return nil
	})

	return err
}

// SetStructMany encodes and stores multiple values using SET commands in one pipeline.
//
// Values are encoded with the client Codec before being stored.
//
// Values are written as independent SET commands, so this helper is safe to use
// with standalone Redis, Redis Cluster, and Ring clients.
//
// For very large input, split items into batches at the call site.
func (c *Client) SetStructMany(ctx context.Context, items []SetItem) error {
	if err := validatePipelineClient(c); err != nil {
		return err
	}

	if len(items) == 0 {
		return nil
	}

	_, err := c.conn.Pipelined(ctx, func(pipe rdb.Pipeliner) error {
		for _, item := range items {
			if item.Expiration < 0 {
				return ErrInvalidTTL
			}

			data, err := c.codec.Marshal(item.Value)
			if err != nil {
				return err
			}

			pipe.Set(ctx, item.Key, data, item.Expiration)
		}

		return nil
	})

	return err
}

// HSetItem describes one Redis HSET operation.
type HSetItem struct {
	// Key is the Redis hash key.
	Key string

	// Values are passed to go-redis HSet.
	//
	// Values may contain flat field-value pairs, maps, structs,
	// or pointers to structs.
	Values []any

	// Expiration is the hash key expiration.
	//
	// Zero leaves the existing expiration unchanged.
	Expiration time.Duration
}

// HSetMany sets fields in multiple Redis hashes using one pipeline.
//
// Each item is written as an independent HSET command.
// If an item has a positive TTL, an EXPIRE command is added for that hash key.
//
// This helper is safe to use with standalone Redis, Redis Cluster,
// and Ring clients because each command operates on one key.
//
// For very large input, split items into batches at the call site.
func (c *Client) HSetMany(ctx context.Context, items []HSetItem) error {
	if err := validatePipelineClient(c); err != nil {
		return err
	}

	if len(items) == 0 {
		return nil
	}

	_, err := c.conn.Pipelined(ctx, func(pipe rdb.Pipeliner) error {
		for _, item := range items {
			if item.Expiration < 0 {
				return ErrInvalidTTL
			}

			if len(item.Values) == 0 {
				return ErrInvalidHashObject
			}

			pipe.HSet(ctx, item.Key, item.Values...)

			if item.Expiration > 0 {
				pipe.Expire(ctx, item.Key, item.Expiration)
			}
		}

		return nil
	})

	return err
}

// DeleteMany deletes keys.
//
// For standalone Redis, keys are deleted using one multi-key DEL command.
// For Redis Cluster and Ring clients, keys are deleted with single-key DEL
// commands inside a pipeline to avoid multi-key hash-slot constraints.
//
// For very large input, split keys into batches at the call site.
func (c *Client) DeleteMany(ctx context.Context, keys []string) error {
	if err := validatePipelineClient(c); err != nil {
		return err
	}

	if len(keys) == 0 {
		return nil
	}

	switch c.conn.(type) {
	case *rdb.ClusterClient, *rdb.Ring:
		_, err := c.conn.Pipelined(ctx, func(pipe rdb.Pipeliner) error {
			for _, key := range keys {
				pipe.Del(ctx, key)
			}

			return nil
		})

		return err

	default:
		return c.conn.Del(ctx, keys...).Err()
	}
}

// UnlinkMany unlinks keys.
//
// UNLINK removes keys from the keyspace and reclaims memory asynchronously,
// which is preferable for large values.
//
// For standalone Redis, keys are unlinked using one multi-key UNLINK command.
// For Redis Cluster and Ring clients, keys are unlinked with single-key UNLINK
// commands inside a pipeline to avoid multi-key hash-slot constraints.
//
// For very large input, split keys into batches at the call site.
func (c *Client) UnlinkMany(ctx context.Context, keys []string) error {
	if err := validatePipelineClient(c); err != nil {
		return err
	}

	if len(keys) == 0 {
		return nil
	}

	switch c.conn.(type) {
	case *rdb.ClusterClient, *rdb.Ring:
		_, err := c.conn.Pipelined(ctx, func(pipe rdb.Pipeliner) error {
			for _, key := range keys {
				pipe.Unlink(ctx, key)
			}

			return nil
		})

		return err

	default:
		return c.conn.Unlink(ctx, keys...).Err()
	}
}

func validatePipelineClient(client *Client) error {
	if client == nil || client.conn == nil {
		return ErrInvalidPipeline
	}

	return nil
}
