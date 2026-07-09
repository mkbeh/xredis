package xredis

import (
	"context"
	"sync"

	rdb "github.com/redis/go-redis/v9"
)

const defaultScanAllCapacity = 1024

// ScanHandler handles one scanned Redis key.
type ScanHandler func(ctx context.Context, key string) error

// ScanBatchHandler handles one scanned Redis key batch.
//
// The batch size follows Redis SCAN behavior. Count is only a hint, so Redis
// does not guarantee that every batch contains exactly Count keys.
type ScanBatchHandler func(ctx context.Context, keys []string) error

// ScanOptions configures Redis SCAN.
type ScanOptions struct {
	// Cursor is the Redis SCAN cursor.
	//
	// Use zero to start a new scan.
	// Cursor is used only by Scan.
	// ScanEach, ScanEachBatch and ScanAll always start from zero.
	Cursor uint64

	// Match filters keys by Redis glob-style pattern.
	//
	// Empty Match scans all keys.
	Match string

	// Count is a scan work hint.
	//
	// Redis does not guarantee that exactly Count keys are returned.
	// If Count is zero, Redis default is used.
	Count int64

	// Type filters keys by Redis data type when supported by Redis.
	//
	// Examples: string, list, set, zset, hash, stream.
	// Empty Type disables type filtering.
	Type string
}

// Scan scans one page of Redis keys.
//
// SCAN is cursor-based. Use cursor=0 to start a new scan.
// The scan is complete when the returned cursor is 0.
//
// Count is a hint, not a strict result limit.
//
// For full scans, prefer ScanEach. For large keyspaces, avoid KEYS.
// For Redis Cluster and Ring clients, use ScanEach or ScanAll for topology-wide scans.
func (c *Client) Scan(ctx context.Context, opts ScanOptions) ([]string, uint64, error) {
	if err := validateScan(c, opts); err != nil {
		return nil, 0, err
	}

	return scanPage(ctx, c.conn, opts)
}

// ScanAll scans all Redis keys matching options and returns them as a slice.
//
// ScanAll always starts from cursor 0. The Cursor option is ignored.
//
// For large keyspaces, prefer ScanEach to avoid storing all keys in memory.
//
// SCAN can return duplicate keys, so this method does not guarantee uniqueness.
func (c *Client) ScanAll(ctx context.Context, opts ScanOptions) ([]string, error) {
	keys := make([]string, 0, defaultScanAllCapacity)

	var mu sync.Mutex

	err := c.ScanEachBatch(ctx, opts, func(_ context.Context, batch []string) error {
		mu.Lock()
		keys = append(keys, batch...)
		mu.Unlock()
		return nil
	})
	if err != nil {
		return nil, err
	}

	return keys, nil
}

// ScanEach scans all Redis keys matching options and calls fn for each key.
//
// ScanEach always starts from cursor 0. The Cursor option is ignored.
// Use Scan for cursor-based pagination.
//
// For Redis Cluster clients, ScanEach scans each master node. Each master node
// is scanned from cursor 0 because SCAN cursors are node-local.
//
// SCAN can return duplicate keys. Handlers should be safe to call more than
// once for the same key.
func (c *Client) ScanEach(ctx context.Context, opts ScanOptions, fn ScanHandler) error {
	if fn == nil {
		return ErrInvalidScan
	}

	return c.ScanEachBatch(ctx, opts, func(ctx context.Context, keys []string) error {
		for _, key := range keys {
			if err := fn(ctx, key); err != nil {
				return err
			}
		}

		return nil
	})
}

// ScanEachBatch scans all Redis keys matching options and calls fn for each batch.
//
// ScanEachBatch always starts from cursor 0. The Cursor option is ignored.
// Use Scan for cursor-based pagination.
//
// For Redis Cluster clients, ScanEachBatch scans each master node.
// For Redis Ring clients, ScanEachBatch scans each live shard.
// Each node is scanned from cursor 0 because SCAN cursors are node-local.
//
// For Redis Cluster and Ring clients, fn may be called concurrently from
// different nodes or shards. Handlers that mutate shared state must synchronize
// access themselves.
//
// SCAN can return duplicate keys. Handlers should be safe to call more than
// once for the same key.
func (c *Client) ScanEachBatch(ctx context.Context, opts ScanOptions, fn ScanBatchHandler) error {
	if err := validateScan(c, opts); err != nil {
		return err
	}

	if fn == nil {
		return ErrInvalidScan
	}

	opts.Cursor = 0

	var forEachNode func(context.Context, func(context.Context, *rdb.Client) error) error

	switch client := c.conn.(type) {
	case *rdb.ClusterClient:
		forEachNode = client.ForEachMaster

	case *rdb.Ring:
		forEachNode = client.ForEachShard

	default:
		return scanNode(ctx, c.conn, opts, fn)
	}

	return forEachNode(ctx, func(nodeCtx context.Context, client *rdb.Client) error {
		nodeOpts := opts
		nodeOpts.Cursor = 0

		return scanNode(nodeCtx, client, nodeOpts, fn)
	})
}

// ScanDelete deletes all Redis keys matching options using DEL.
//
// For Redis Cluster and Ring clients, deletion is executed as pipelined
// single-key DEL commands to avoid multi-key hash-slot constraints.
func (c *Client) ScanDelete(ctx context.Context, opts ScanOptions) error {
	return c.ScanEachBatch(ctx, opts, func(ctx context.Context, keys []string) error {
		return c.DeleteMany(ctx, keys)
	})
}

// ScanUnlink unlinks all Redis keys matching options using UNLINK.
//
// UNLINK removes keys from the keyspace and reclaims memory asynchronously,
// which is preferable for large values.
//
// For Redis Cluster and Ring clients, unlinking is executed as pipelined
// single-key UNLINK commands to avoid multi-key hash-slot constraints.
func (c *Client) ScanUnlink(ctx context.Context, opts ScanOptions) error {
	return c.ScanEachBatch(ctx, opts, func(ctx context.Context, keys []string) error {
		return c.UnlinkMany(ctx, keys)
	})
}

type keyScanner interface {
	Scan(ctx context.Context, cursor uint64, match string, count int64) *rdb.ScanCmd
	ScanType(ctx context.Context, cursor uint64, match string, count int64, keyType string) *rdb.ScanCmd
}

func scanNode(
	ctx context.Context,
	client keyScanner,
	opts ScanOptions,
	fn ScanBatchHandler,
) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		keys, nextCursor, err := scanPage(ctx, client, opts)
		if err != nil {
			return err
		}

		if len(keys) > 0 {
			if err := fn(ctx, keys); err != nil {
				return err
			}
		}

		if nextCursor == 0 {
			return nil
		}

		opts.Cursor = nextCursor
	}
}

func scanPage(ctx context.Context, client keyScanner, opts ScanOptions) ([]string, uint64, error) {
	var cmd *rdb.ScanCmd

	if opts.Type != "" {
		cmd = client.ScanType(ctx, opts.Cursor, opts.Match, opts.Count, opts.Type)
	} else {
		cmd = client.Scan(ctx, opts.Cursor, opts.Match, opts.Count)
	}

	return cmd.Result()
}

func validateScan(client *Client, opts ScanOptions) error {
	if client == nil || client.conn == nil {
		return ErrInvalidScan
	}

	if opts.Count < 0 {
		return ErrInvalidScan
	}

	return nil
}
