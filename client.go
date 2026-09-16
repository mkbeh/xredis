package xredis

import (
	"context"

	rdb "github.com/redis/go-redis/v9"
)

// Client is an opinionated Redis client wrapper.
type Client struct {
	conn    rdb.UniversalClient
	codec   Codec
	metrics clientMetrics
}

// NewClient creates a standalone Redis client.
func NewClient(redisOpts *rdb.Options, opts ...Option) (*Client, error) {
	if redisOpts == nil {
		return nil, ErrInvalidOptions
	}

	return newClient(rdb.NewClient(redisOpts), newOptions(opts...)), nil
}

// NewClusterClient creates a Redis Cluster client.
func NewClusterClient(redisOpts *rdb.ClusterOptions, opts ...Option) (*Client, error) {
	if redisOpts == nil {
		return nil, ErrInvalidOptions
	}

	return newClient(rdb.NewClusterClient(redisOpts), newOptions(opts...)), nil
}

// NewFailoverClient creates a Redis Sentinel / failover client.
func NewFailoverClient(redisOpts *rdb.FailoverOptions, opts ...Option) (*Client, error) {
	if redisOpts == nil {
		return nil, ErrInvalidOptions
	}

	return newClient(rdb.NewFailoverClient(redisOpts), newOptions(opts...)), nil
}

// NewFailoverClusterClient creates a Redis Sentinel / failover cluster client.
func NewFailoverClusterClient(redisOpts *rdb.FailoverOptions, opts ...Option) (*Client, error) {
	if redisOpts == nil {
		return nil, ErrInvalidOptions
	}

	return newClient(rdb.NewFailoverClusterClient(redisOpts), newOptions(opts...)), nil
}

// NewRing creates a Redis Ring client for client-side sharding.
func NewRing(redisOpts *rdb.RingOptions, opts ...Option) (*Client, error) {
	if redisOpts == nil {
		return nil, ErrInvalidOptions
	}

	return newClient(rdb.NewRing(redisOpts), newOptions(opts...)), nil
}

// Raw returns the underlying go-redis client.
func (c *Client) Raw() rdb.UniversalClient {
	return c.conn
}

// Ping checks Redis availability.
func (c *Client) Ping(ctx context.Context) error {
	return c.conn.Ping(ctx).Err()
}

// Close closes the Redis client.
func (c *Client) Close() error {
	return c.conn.Close()
}

func newClient(conn rdb.UniversalClient, opts options) *Client {
	client := &Client{
		conn:  conn,
		codec: opts.codec,
	}

	if opts.metrics != nil {
		client.metrics = newClientMetrics(opts.metrics.Register())
	}

	return client
}
