package xredis

import (
	"context"

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

type scanRemoveMode uint8

const (
	scanRemoveModeDelete scanRemoveMode = iota
	scanRemoveModeUnlink
)

// ScanOptions configures Redis SCAN.
type ScanOptions struct {
	// Cursor is the Redis SCAN cursor.
	//
	// Use zero to start a new scan.
	// Cursor is used only by Scan. ScanEach and ScanAll always start from zero.
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
// For Redis Cluster clients, use ScanEach or ScanAll for cluster-wide scans.
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

	err := c.ScanEachBatch(ctx, opts, func(_ context.Context, batch []string) error {
		keys = append(keys, batch...)
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
// For Redis Cluster clients, ScanEachBatch scans each master node. Each master
// node is scanned from cursor 0 because SCAN cursors are node-local.
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

	scanOpts := opts
	scanOpts.Cursor = 0

	if cluster, ok := c.conn.(*rdb.ClusterClient); ok {
		return cluster.ForEachMaster(ctx, func(ctx context.Context, client *rdb.Client) error {
			nodeOpts := scanOpts
			nodeOpts.Cursor = 0

			return scanNode(ctx, client, nodeOpts, fn)
		})
	}

	return scanNode(ctx, c.conn, scanOpts, fn)
}

// ScanDelete deletes all Redis keys matching options using DEL.
//
// For Redis Cluster clients, deletion is executed as pipelined single-key DEL
// commands to avoid multi-key hash-slot constraints.
func (c *Client) ScanDelete(ctx context.Context, opts ScanOptions) error {
	return c.scanRemove(ctx, opts, scanRemoveModeDelete)
}

// ScanUnlink unlinks all Redis keys matching options using UNLINK.
//
// UNLINK removes keys from the keyspace and reclaims memory asynchronously,
// which is preferable for large values.
//
// For Redis Cluster clients, unlinking is executed as pipelined single-key
// UNLINK commands to avoid multi-key hash-slot constraints.
func (c *Client) ScanUnlink(ctx context.Context, opts ScanOptions) error {
	return c.scanRemove(ctx, opts, scanRemoveModeUnlink)
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
	cursor := opts.Cursor

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		pageOpts := opts
		pageOpts.Cursor = cursor

		keys, nextCursor, err := scanPage(ctx, client, pageOpts)
		if err != nil {
			return err
		}

		if len(keys) > 0 {
			if err = fn(ctx, keys); err != nil {
				return err
			}
		}

		if nextCursor == 0 {
			return nil
		}

		cursor = nextCursor
	}
}

func scanPage(ctx context.Context, client keyScanner, opts ScanOptions) ([]string, uint64, error) {
	if opts.Type != "" {
		return client.ScanType(ctx, opts.Cursor, opts.Match, opts.Count, opts.Type).Result()
	}

	return client.Scan(ctx, opts.Cursor, opts.Match, opts.Count).Result()
}

func (c *Client) scanRemove(ctx context.Context, opts ScanOptions, mode scanRemoveMode) error {
	return c.ScanEachBatch(ctx, opts, func(ctx context.Context, keys []string) error {
		return c.scanRemoveBatch(ctx, keys, mode)
	})
}

func (c *Client) scanRemoveBatch(ctx context.Context, keys []string, mode scanRemoveMode) error {
	if len(keys) == 0 {
		return nil
	}

	_, err := c.conn.Pipelined(ctx, func(pipe rdb.Pipeliner) error {
		for _, key := range keys {
			switch mode {
			case scanRemoveModeDelete:
				pipe.Del(ctx, key)
			case scanRemoveModeUnlink:
				pipe.Unlink(ctx, key)
			}
		}

		return nil
	})

	return err
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
