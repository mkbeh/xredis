package redis

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"log/slog"
	"reflect"
	"time"

	"github.com/redis/go-redis/extra/redisotel/v9"
	rdb "github.com/redis/go-redis/v9"
)

type Client struct {
	id           string
	suffix       string
	conn         rdb.UniversalClient
	cfg          *Config
	tls          *tls.Config
	logger       *slog.Logger
	limiter      rdb.Limiter
	marshaller   MarshallerFunc
	meterOptions []redisotel.MetricsOption
	traceOptions []redisotel.TracingOption
}

func NewClient(opts ...Option) (*Client, error) {
	return newClient(false, opts)
}

func NewClusterClient(opts ...Option) (*Client, error) {
	return newClient(true, opts)
}

func newClient(cluster bool, opts []Option) (*Client, error) {
	c := &Client{
		cfg:    defaultConfig(),
		logger: slog.Default(),
	}

	for _, opt := range opts {
		opt.apply(c)
	}

	c.logger = c.logger.With(slog.String("component", "redis"))

	if c.marshaller == nil {
		c.marshaller = json.Marshal
	}

	if cluster {
		connOpts := parseClusterConfig(c.cfg)
		connOpts.TLSConfig = c.tls
		connOpts.ClientName = c.getID()
		c.conn = rdb.NewClusterClient(connOpts)
	} else {
		connOpts := parseConfig(c.cfg)
		connOpts.TLSConfig = c.tls
		connOpts.ClientName = c.getID()
		connOpts.Limiter = c.limiter
		c.conn = rdb.NewClient(connOpts)
	}

	if err := c.exposeInstrumenting(); err != nil {
		return nil, err
	}

	if err := c.conn.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}

	return c, nil
}

// Exists returns if key exists.
func (c *Client) Exists(ctx context.Context, key string) (exists bool, err error) {
	res := c.conn.Exists(ctx, key)
	if err = res.Err(); err != nil {
		return
	}

	exists = res.Val() == 1
	return
}

// HExists returns if field is an existing field in the hash stored at key.
func (c *Client) HExists(ctx context.Context, key, field string) (exists bool, err error) {
	return c.conn.HExists(ctx, key, field).Result()
}

// HIncrBy increments the number stored at field in the hash stored at key by increment.
func (c *Client) HIncrBy(ctx context.Context, key, field string, incr int64) (err error) {
	return c.conn.HIncrBy(ctx, key, field, incr).Err()
}

// HGetAll returns all fields and values of the hash stored at key and scans the result into dst variable.
func (c *Client) HGetAll(ctx context.Context, key string, dst any) (err error) {
	res := c.conn.HGetAll(ctx, key)
	if err = res.Err(); err != nil {
		return
	}

	if len(res.Val()) == 0 {
		return ErrKeyNotFound
	}

	return res.Scan(dst)
}

// HGet returns the value associated with field in the hash stored at key and scan the result into dst variable.
func (c *Client) HGet(ctx context.Context, key, field string, dst any) (err error) {
	if err = c.conn.HGet(ctx, key, field).Scan(dst); err != nil {
		if errors.Is(err, rdb.Nil) {
			return ErrKeyNotFound
		}
		return
	}
	return
}

// HSet sets field in the hash stored at key to value and its expiration if expiration wasn't set before.
func (c *Client) HSet(ctx context.Context, key, field string, value any, ttl time.Duration) (err error) {
	pipe := c.conn.TxPipeline()

	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Invalid, reflect.Pointer, reflect.Array, reflect.Map, reflect.Struct,
		reflect.Slice, reflect.Func, reflect.Chan, reflect.UnsafePointer:
		err = ErrInvalidFieldType
		return
	default:
	}

	pipe.HSet(ctx, key, field, value)
	pipe.Expire(ctx, key, ttl)

	cmder, err := pipe.Exec(ctx)
	if err != nil {
		return
	}

	for _, cmd := range cmder {
		if err = cmd.Err(); err != nil {
			return
		}
	}

	return
}

// HSetObject sets fields of data object in the hash stored at key to value.
func (c *Client) HSetObject(ctx context.Context, key string, data any, ttl time.Duration) (err error) {
	pipe := c.conn.TxPipeline()

	t := reflect.TypeOf(data)
	v := reflect.ValueOf(data)
	for i := 0; i < t.NumField(); i++ {
		tagValue, ok := t.Field(i).Tag.Lookup("redis")
		if ok && tagValue != "-" {
			switch v.Field(i).Kind() {
			case reflect.Invalid, reflect.Pointer, reflect.Array, reflect.Map, reflect.Struct,
				reflect.Slice, reflect.Func, reflect.Chan, reflect.UnsafePointer:
				return ErrInvalidFieldType
			default:
			}
			pipe.HSet(ctx, key, tagValue, v.Field(i).Interface())
		}
	}

	pipe.Expire(ctx, key, ttl)

	cmder, err := pipe.Exec(ctx)
	if err != nil {
		return
	}

	for _, cmd := range cmder {
		if err = cmd.Err(); err != nil {
			return
		}
	}

	return
}

func (c *Client) Get(ctx context.Context, key string, dst any) (err error) {
	if err = c.conn.Get(ctx, key).Scan(dst); err != nil {
		if errors.Is(err, rdb.Nil) {
			return ErrKeyNotFound
		}
		return
	}
	return
}

// Set Redis `SET key value [expiration]` command.
// Use for single items.
func (c *Client) Set(ctx context.Context, key string, val interface{}, expiration time.Duration) (err error) {
	res := c.conn.Set(ctx, key, val, expiration)
	err = res.Err()
	if err != nil {
		return
	}
	return
}

func (c *Client) SetStruct(ctx context.Context, key string, val interface{}, expiration time.Duration) (err error) {
	b, err := c.marshaller(val)
	if err != nil {
		return err
	}
	res := c.conn.Set(ctx, key, b, expiration)
	err = res.Err()
	if err != nil {
		return
	}
	return
}

func (c *Client) Bool(ctx context.Context, key string) (val, ok bool, err error) {
	res := c.conn.Get(ctx, key)
	val, err = res.Bool()
	if err != nil {
		if errors.Is(err, rdb.Nil) {
			err = nil
			return
		}
		return
	}
	ok = true
	return
}

func (c *Client) Bytes(ctx context.Context, key string) (val []byte, ok bool, err error) {
	res := c.conn.Get(ctx, key)
	val, err = res.Bytes()
	if err != nil {
		if errors.Is(err, rdb.Nil) {
			err = nil
			return
		}
		return
	}
	ok = true
	return
}

func (c *Client) Float64(ctx context.Context, key string) (val float64, ok bool, err error) {
	res := c.conn.Get(ctx, key)
	val, err = res.Float64()
	if err != nil {
		if errors.Is(err, rdb.Nil) {
			err = nil
			return
		}
		return
	}
	ok = true
	return
}

func (c *Client) Int(ctx context.Context, key string) (val int, ok bool, err error) {
	res := c.conn.Get(ctx, key)
	val, err = res.Int()
	if err != nil {
		if errors.Is(err, rdb.Nil) {
			err = nil
			return
		}
		return
	}
	ok = true
	return
}

func (c *Client) Int64(ctx context.Context, key string) (val int64, ok bool, err error) {
	res := c.conn.Get(ctx, key)
	val, err = res.Int64()
	if err != nil {
		if errors.Is(err, rdb.Nil) {
			err = nil
			return
		}
		return
	}
	ok = true
	return
}

func (c *Client) Uint64(ctx context.Context, key string) (val uint64, ok bool, err error) {
	res := c.conn.Get(ctx, key)
	val, err = res.Uint64()
	if err != nil {
		if errors.Is(err, rdb.Nil) {
			err = nil
			return
		}
		return
	}
	ok = true
	return
}

// Get Redis `GET key` command. It returns string. Return error when key does not exist.
func (c *Client) String(ctx context.Context, key string) (val string, ok bool, err error) {
	res := c.conn.Get(ctx, key)
	val, err = res.Result()
	if err != nil {
		if errors.Is(err, rdb.Nil) {
			err = nil
			return
		}
		return
	}
	ok = true
	return
}

func (c *Client) Incr(ctx context.Context, key string) (err error) {
	res := c.conn.Incr(ctx, key)
	err = res.Err()
	if err != nil {
		return
	}
	return
}

func (c *Client) Decr(ctx context.Context, key string) (err error) {
	res := c.conn.Decr(ctx, key)
	err = res.Err()
	if err != nil {
		return
	}
	return
}

func (c *Client) Delete(ctx context.Context, key string) (err error) {
	res := c.conn.Del(ctx, key)
	err = res.Err()
	if err != nil {
		return
	}
	return
}

// MassDelete realise pipeline mass delete values by key slice.
func (c *Client) MassDelete(ctx context.Context, keys []string) (err error) {
	cmders, err := c.conn.Pipelined(ctx, func(pipe rdb.Pipeliner) error {
		pipe.Del(ctx, keys...)
		return nil
	})

	for _, cmder := range cmders {
		err = cmder.Err()
		if err != nil {
			return
		}
	}
	return
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) getID() string {
	if c.id == "" {
		return GenerateUUID()
	}
	return c.id
}

func (c *Client) addMeterOption(opt redisotel.MetricsOption) {
	c.meterOptions = append(c.meterOptions, opt)
}

func (c *Client) addTraceOption(opt redisotel.TracingOption) {
	c.traceOptions = append(c.traceOptions, opt)
}

func (c *Client) exposeInstrumenting() error {
	err := redisotel.InstrumentTracing(c.conn, c.traceOptions...)
	if err != nil {
		return err
	}
	err = redisotel.InstrumentMetrics(c.conn, c.meterOptions...)
	if err != nil {
		return err
	}
	return nil
}
