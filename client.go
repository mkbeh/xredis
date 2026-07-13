package xredis

import (
	"context"

	"github.com/redis/go-redis/extra/redisotel/v9"
	rdb "github.com/redis/go-redis/v9"
)

// Client is an opinionated Redis client wrapper.
type Client struct {
	conn    rdb.UniversalClient
	codec   Codec
	metrics *metrics
}

// NewClient creates a standalone Redis client.
func NewClient(opts ...Option) (*Client, error) {
	options := newOptions(opts...)

	redisOpts, err := options.clientOptions()
	if err != nil {
		return nil, err
	}

	return newClient(rdb.NewClient(redisOpts), options)
}

// NewClusterClient creates a Redis Cluster client.
func NewClusterClient(opts ...Option) (*Client, error) {
	options := newOptions(opts...)

	redisOpts, err := options.clusterOptions()
	if err != nil {
		return nil, err
	}

	return newClient(rdb.NewClusterClient(redisOpts), options)
}

// NewFailoverClient creates a Redis Sentinel / failover client.
func NewFailoverClient(opts ...Option) (*Client, error) {
	options := newOptions(opts...)

	redisOpts, err := options.failoverOptions()
	if err != nil {
		return nil, err
	}

	return newClient(rdb.NewFailoverClient(redisOpts), options)
}

// NewFailoverClusterClient creates a Redis Sentinel / failover cluster client.
func NewFailoverClusterClient(opts ...Option) (*Client, error) {
	options := newOptions(opts...)

	redisOpts, err := options.failoverOptions()
	if err != nil {
		return nil, err
	}

	return newClient(rdb.NewFailoverClusterClient(redisOpts), options)
}

// NewRing creates a Redis Ring client for client-side sharding.
func NewRing(opts ...Option) (*Client, error) {
	options := newOptions(opts...)

	redisOpts, err := options.ringOptions()
	if err != nil {
		return nil, err
	}

	return newClient(rdb.NewRing(redisOpts), options)
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

func newClient(conn rdb.UniversalClient, opts *options) (*Client, error) {
	if err := applyTracing(conn, opts.traceOptions); err != nil {
		_ = conn.Close()
		return nil, err
	}

	return &Client{
		conn:    conn,
		codec:   opts.codec,
		metrics: newClientMetrics(opts.metricLabels),
	}, nil
}

func applyTracing(conn rdb.UniversalClient, traceOptions []redisotel.TracingOption) error {
	if len(traceOptions) == 0 {
		return nil
	}

	return redisotel.InstrumentTracing(conn, traceOptions...)
}
