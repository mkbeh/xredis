package xredis

import (
	"context"
	"time"

	rdb "github.com/redis/go-redis/v9"
)

// SetItem describes one codec-based Redis SET operation.
type SetItem struct {
	// Key is the Redis key.
	Key string

	// Value is encoded with the client Codec before being stored.
	Value any

	// Expiration is the key expiration.
	//
	// Zero means no expiration.
	Expiration time.Duration
}

// SetMany encodes and stores multiple values using SET commands in one pipeline.
//
// Values are encoded with the client Codec.
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
