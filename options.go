package redis

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/redis/go-redis/extra/redisotel/v9"
	rdb "github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// An Option lets you add opts using With* funcs.
type Option interface {
	apply(p *Client)
}

type optionFunc func(c *Client)

func (f optionFunc) apply(c *Client) {
	f(c)
}

type MarshallerFunc func(interface{}) ([]byte, error)

func WithLogger(l *slog.Logger) Option {
	return optionFunc(func(c *Client) {
		if l != nil {
			c.logger = l
		}
	})
}

func WithConfig(config *Config) Option {
	return optionFunc(func(c *Client) {
		if config != nil {
			c.cfg = config
		}
	})
}

func WithClientID(id string) Option {
	return optionFunc(func(c *Client) {
		if id != "" {
			c.id = fmt.Sprintf("%s-%s", id, GenerateUUID())
		}
	})
}

func WithIdentitySuffix(suffix string) Option {
	return optionFunc(func(c *Client) {
		if suffix != "" {
			c.suffix = suffix
		}
	})
}

func WithMarshaller(fn MarshallerFunc) Option {
	return optionFunc(func(c *Client) {
		if fn != nil {
			c.marshaller = fn
		}
	})
}

func WithTLS(cfg *tls.Config) Option {
	return optionFunc(func(c *Client) {
		if cfg != nil {
			c.tls = cfg
		}
	})
}

func WithLimiter(limiter rdb.Limiter) Option {
	return optionFunc(func(c *Client) {
		if limiter != nil {
			c.limiter = limiter
		}
	})
}

// --- metrics ---

func WithMeterProvider(mp metric.MeterProvider) Option {
	return optionFunc(func(c *Client) {
		if mp != nil {
			c.addMeterOption(redisotel.WithMeterProvider(mp))
		}
	})
}

func WithMetricsNamespace(namespace string) Option {
	return optionFunc(func(c *Client) {
		if namespace != "" {
			c.namespace = namespace
		}
	})
}

// --- tracing ---

func WithTraceProvider(provider trace.TracerProvider) Option {
	return optionFunc(func(c *Client) {
		if provider != nil {
			c.addTraceOption(redisotel.WithTracerProvider(provider))
		}
	})
}

// WithDBStatement tells the tracing hook not to log raw redis commands.
func WithDBStatement(on bool) Option {
	return optionFunc(func(c *Client) {
		c.addTraceOption(redisotel.WithDBStatement(on))
	})
}

func WithDBSystem(system string) Option {
	return optionFunc(func(c *Client) {
		if system != "" {
			c.addTraceOption(redisotel.WithDBSystem(system))
			c.addMeterOption(redisotel.WithDBSystem(system))
		}
	})
}

// WithAttributes specifies additional attributes to be added to the span.
func WithAttributes(attrs ...attribute.KeyValue) Option {
	return optionFunc(func(c *Client) {
		c.addTraceOption(redisotel.WithAttributes(attrs...))
		c.addMeterOption(redisotel.WithAttributes(attrs...))
	})
}

type Config struct {
	// The network type, either tcp or unix.
	// Default is tcp.
	Network string `envconfig:"REDIS_NETWORK"`

	// A seed list of host:port addresses of cluster nodes.
	Addrs string `envconfig:"REDIS_ADDRS"`

	// Protocol 2 or 3. Use the version to negotiate RESP version with redis-server.
	// Default is 3.
	Protocol int `envconfig:"REDIS_PROTOCOL"`
	// Use the specified Username to authenticate the current connection
	// with one of the connections defined in the ACL list when connecting
	// to a Redis 6.0 instance, or greater, that is using the Redis ACL system.
	Username string `envconfig:"REDIS_USERNAME"`
	// Optional password. Must match the password specified in the
	// requirepass server configuration option (if connecting to a Redis 5.0 instance, or lower),
	// or the User Password when connecting to a Redis 6.0 instance, or greater,
	// that is using the Redis ACL system.
	Password string `envconfig:"REDIS_PASSWORD"`

	// Database to be selected after connecting to the server.
	DB int `envconfig:"REDIS_DB"`

	// The maximum number of retries before giving up. Command is retried
	// on network errors and MOVED/ASK redirects.
	// Default is 3 retries.
	MaxRedirects int `envconfig:"REDIS_MAX_REDIRECTS"`

	// Enables read-only commands on slave nodes.
	ReadOnly bool `envconfig:"REDIS_READONLY"`
	// Allows routing read-only commands to the closest master or slave node.
	// It automatically enables ReadOnly.
	RouteByLatency bool `envconfig:"REDIS_ROUTE_BY_LATENCY"`
	// Allows routing read-only commands to the random master or slave node.
	// It automatically enables ReadOnly.
	RouteRandomly bool `envconfig:"REDIS_ROUTE_RANDOMLY"`

	// Maximum number of retries before giving up.
	// Default is 3 retries; -1 (not 0) disables retries.
	MaxRetries int `envconfig:"REDIS_MAX_RETRIES"`
	// Minimum backoff between each retry.
	// Default is 8 milliseconds; -1 disables backoff.
	MinRetryBackoff time.Duration `envconfig:"REDIS_MIN_RETRY_BACKOFF"`
	// Maximum backoff between each retry.
	// Default is 512 milliseconds; -1 disables backoff.
	MaxRetryBackoff time.Duration `envconfig:"REDIS_MAX_RETRY_BACKOFF"`

	// Dial timeout for establishing new connections.
	// Default is 5 seconds.
	DialTimeout time.Duration `envconfig:"REDIS_DIAL_TIMEOUT"`
	// Timeout for socket reads. If reached, commands will fail
	// with a timeout instead of blocking. Supported values:
	//   - `0` - default timeout (3 seconds).
	//   - `-1` - no timeout (block indefinitely).
	//   - `-2` - disables SetReadDeadline calls completely.
	ReadTimeout time.Duration `envconfig:"REDIS_READ_TIMEOUT"`
	// Timeout for socket writes. If reached, commands will fail
	// with a timeout instead of blocking.  Supported values:
	//   - `0` - default timeout (3 seconds).
	//   - `-1` - no timeout (block indefinitely).
	//   - `-2` - disables SetWriteDeadline calls completely.
	WriteTimeout time.Duration `envconfig:"REDIS_WRITE_TIMEOUT"`
	// ContextTimeoutEnabled controls whether the client respects context timeouts and deadlines.
	// See https://redis.uptrace.dev/guide/go-redis-debugging.html#timeouts
	ContextTimeoutEnabled bool `envconfig:"REDIS_CONTEXT_TIMEOUT_ENABLED"`

	// Type of connection pool.
	// true for FIFO pool, false for LIFO pool.
	// Note that FIFO has slightly higher overhead compared to LIFO,
	// but it helps closing idle connections faster reducing the pool size.
	PoolFIFO bool `envconfig:"REDIS_POOL_FIFO"`
	// Base number of socket connections.
	// Default is 10 connections per every available CPU as reported by runtime.GOMAXPROCS.
	// If there is not enough connections in the pool, new connections will be allocated in excess of PoolSize,
	// you can limit it through MaxActiveConns
	PoolSize int `envconfig:"REDIS_POOL_SIZE"` // applies per cluster node and not for the whole cluster
	// Amount of time client waits for connection if all connections
	// are busy before returning an error.
	// Default is ReadTimeout + 1 second.
	PoolTimeout time.Duration `envconfig:"REDIS_POOL_TIMEOUT"`
	// Minimum number of idle connections which is useful when establishing
	// new connection is slow.
	// Default is 0. the idle connections are not closed by default.
	MinIdleConns int `envconfig:"REDIS_MIN_IDLE_CONNS"`
	// Maximum number of idle connections.
	// Default is 0. the idle connections are not closed by default.
	MaxIdleConns int `envconfig:"REDIS_MAX_IDLE_CONNS"`
	// Maximum number of connections allocated by the pool at a given time.
	// When zero, there is no limit on the number of connections in the pool.
	MaxActiveConns int `envconfig:"REDIS_MAX_ACTIVE_CONNS"` // applies per cluster node and not for the whole cluster
	// ConnMaxIdleTime is the maximum amount of time a connection may be idle.
	// Should be less than server's timeout.
	//
	// Expired connections may be closed lazily before reuse.
	// If d <= 0, connections are not closed due to a connection's idle time.
	//
	// Default is 30 minutes. -1 disables idle timeout check.
	ConnMaxIdleTime time.Duration `envconfig:"REDIS_CONN_MAX_IDLE_TIME"`
	// ConnMaxLifetime is the maximum amount of time a connection may be reused.
	//
	// Expired connections may be closed lazily before reuse.
	// If <= 0, connections are not closed due to a connection's age.
	//
	// Default is to not close idle connections.
	ConnMaxLifetime time.Duration `envconfig:"REDIS_CONN_MAX_LIFETIME"`

	// Disable set-lib on connect. Default is false.
	DisableIndentity bool `envconfig:"REDIS_DISABLE_INDENTITY"`

	// Enable Unstable mode for Redis Search module with RESP3.
	UnstableResp3 bool `envconfig:"REDIS_UNSTABLE_RESP3"`
}

func parseClientConfig(cfg *Config) *rdb.Options {
	opts := &rdb.Options{
		Network:               cfg.Network,
		Username:              cfg.Username,
		Password:              cfg.Password,
		DB:                    cfg.DB,
		MaxRetries:            cfg.MaxRetries,
		MinRetryBackoff:       cfg.MinRetryBackoff,
		MaxRetryBackoff:       cfg.MaxRetryBackoff,
		DialTimeout:           cfg.DialTimeout,
		ReadTimeout:           cfg.ReadTimeout,
		WriteTimeout:          cfg.WriteTimeout,
		ContextTimeoutEnabled: cfg.ContextTimeoutEnabled,
		PoolFIFO:              cfg.PoolFIFO,
		PoolSize:              cfg.PoolSize,
		PoolTimeout:           cfg.PoolTimeout,
		MinIdleConns:          cfg.MinIdleConns,
		MaxIdleConns:          cfg.MaxIdleConns,
		MaxActiveConns:        cfg.MaxActiveConns,
		ConnMaxIdleTime:       cfg.ConnMaxIdleTime,
		ConnMaxLifetime:       cfg.ConnMaxLifetime,
		DisableIndentity:      cfg.DisableIndentity,
		UnstableResp3:         cfg.UnstableResp3,
	}

	addrs := strings.Split(cfg.Addrs, ",")
	if len(addrs) > 0 {
		opts.Addr = addrs[0]
	}

	if cfg.Protocol > 0 {
		opts.Protocol = cfg.Protocol
	}

	return opts
}

func parseClusterConfig(cfg *Config) *rdb.ClusterOptions {
	opts := &rdb.ClusterOptions{
		Addrs:                 strings.Split(cfg.Addrs, ","),
		Username:              cfg.Username,
		Password:              cfg.Password,
		RouteByLatency:        cfg.RouteByLatency,
		RouteRandomly:         cfg.RouteRandomly,
		ReadOnly:              cfg.ReadOnly,
		MaxRetries:            cfg.MaxRetries,
		MinRetryBackoff:       cfg.MinRetryBackoff,
		MaxRetryBackoff:       cfg.MaxRetryBackoff,
		DialTimeout:           cfg.DialTimeout,
		ReadTimeout:           cfg.ReadTimeout,
		WriteTimeout:          cfg.WriteTimeout,
		ContextTimeoutEnabled: cfg.ContextTimeoutEnabled,
		PoolFIFO:              cfg.PoolFIFO,
		PoolSize:              cfg.PoolSize,
		PoolTimeout:           cfg.PoolTimeout,
		MinIdleConns:          cfg.MinIdleConns,
		MaxIdleConns:          cfg.MaxIdleConns,
		MaxActiveConns:        cfg.MaxActiveConns,
		ConnMaxIdleTime:       cfg.ConnMaxIdleTime,
		ConnMaxLifetime:       cfg.ConnMaxLifetime,
		DisableIndentity:      cfg.DisableIndentity,
	}

	if cfg.Protocol > 0 {
		opts.Protocol = cfg.Protocol
	}

	if cfg.MaxRedirects == 0 {
		opts.MaxRedirects = len(cfg.Addrs) + 1 // set the number of retries and redirects according to the number of nodes +1
	}

	return opts
}

func defaultConfig() *Config {
	return &Config{
		Addrs:    "127.0.0.1:6379",
		ReadOnly: true,
	}
}
