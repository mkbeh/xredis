package xredis

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"time"

	rdb "github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/auth"
	"github.com/redis/go-redis/v9/push"
)

// Option configures xredis Client construction.
type Option interface {
	apply(opts *options)
}

type optionFunc func(opts *options)

func (f optionFunc) apply(opts *options) {
	f(opts)
}

type options struct {
	cfg any

	// Client identity.
	clientName     string
	identitySuffix string

	// Runtime dependencies.
	tls         *tls.Config
	limiter     rdb.Limiter
	codec       Codec
	credentials credentialsOptions

	// Connection hooks.
	dialer             func(ctx context.Context, network, addr string) (net.Conn, error)
	onConnect          func(ctx context.Context, cn *rdb.Conn) error
	dialerRetryBackoff func(attempt int) time.Duration

	// Cluster hooks.
	clusterNewClient func(opt *rdb.Options) *rdb.Client
	clusterSlots     func(context.Context) ([]rdb.ClusterSlot, error)

	// Ring hooks.
	ringNewClient      func(opt *rdb.Options) *rdb.Client
	ringHeartbeatFn    func(ctx context.Context, client *rdb.Client) bool
	ringConsistentHash func(shards []string) rdb.ConsistentHash

	// Push notifications.
	pushNotificationProcessor push.NotificationProcessor

	// Observability.
	metrics Metrics
	tracing Tracing
}

type credentialsOptions struct {
	provider          func() (username, password string)
	providerContext   func(ctx context.Context) (username, password string, err error)
	streamingProvider auth.StreamingCredentialsProvider
}

func newOptions(opts ...Option) *options {
	options := &options{
		codec: JSONCodec{},
	}

	for _, opt := range opts {
		if opt != nil {
			opt.apply(options)
		}
	}

	if options.codec == nil {
		options.codec = JSONCodec{}
	}

	return options
}

func (o *options) clientOptions() (*rdb.Options, error) {
	cfg, ok := o.cfg.(*ClientConfig)
	if o.cfg != nil && (!ok || cfg == nil) {
		return nil, fmt.Errorf("%w: standalone config is required", ErrInvalidConfig)
	}

	if cfg == nil {
		cfg = &ClientConfig{}
	}

	redisOpts, err := parseClientConfig(cfg)
	if err != nil {
		return nil, err
	}

	applyClientOptions(redisOpts, o)

	return redisOpts, nil
}

func (o *options) clusterOptions() (*rdb.ClusterOptions, error) {
	cfg, ok := o.cfg.(*ClusterConfig)
	if !ok || cfg == nil {
		return nil, fmt.Errorf("%w: cluster config is required", ErrInvalidConfig)
	}

	redisOpts, err := parseClusterConfig(cfg)
	if err != nil {
		return nil, err
	}

	applyClusterOptions(redisOpts, o)

	return redisOpts, nil
}

func (o *options) failoverOptions() (*rdb.FailoverOptions, error) {
	cfg, ok := o.cfg.(*FailoverConfig)
	if !ok || cfg == nil {
		return nil, fmt.Errorf("%w: failover config is required", ErrInvalidConfig)
	}

	redisOpts, err := parseFailoverConfig(cfg)
	if err != nil {
		return nil, err
	}

	applyFailoverOptions(redisOpts, o)

	return redisOpts, nil
}

func (o *options) ringOptions() (*rdb.RingOptions, error) {
	cfg, ok := o.cfg.(*RingConfig)
	if !ok || cfg == nil {
		return nil, fmt.Errorf("%w: ring config is required", ErrInvalidConfig)
	}

	redisOpts, err := parseRingConfig(cfg)
	if err != nil {
		return nil, err
	}

	applyRingOptions(redisOpts, o)

	return redisOpts, nil
}

// Config options.

// WithClientConfig configures standalone Redis client options.
func WithClientConfig(cfg *ClientConfig) Option {
	return optionFunc(func(opts *options) {
		if cfg != nil {
			opts.cfg = cfg
		}
	})
}

// WithClusterConfig configures Redis Cluster client options.
func WithClusterConfig(cfg *ClusterConfig) Option {
	return optionFunc(func(opts *options) {
		if cfg != nil {
			opts.cfg = cfg
		}
	})
}

// WithFailoverConfig configures Redis Sentinel / failover client options.
func WithFailoverConfig(cfg *FailoverConfig) Option {
	return optionFunc(func(opts *options) {
		if cfg != nil {
			opts.cfg = cfg
		}
	})
}

// WithRingConfig configures Redis Ring client options.
func WithRingConfig(cfg *RingConfig) Option {
	return optionFunc(func(opts *options) {
		if cfg != nil {
			opts.cfg = cfg
		}
	})
}

// Identity options.

// WithClientName configures the Redis client name.
func WithClientName(name string) Option {
	return optionFunc(func(opts *options) {
		if name != "" {
			opts.clientName = name
		}
	})
}

// WithIdentitySuffix configures go-redis identity suffix.
func WithIdentitySuffix(suffix string) Option {
	return optionFunc(func(opts *options) {
		if suffix != "" {
			opts.identitySuffix = suffix
		}
	})
}

// Encoding options.

// WithCodec configures value codec.
func WithCodec(codec Codec) Option {
	return optionFunc(func(opts *options) {
		if codec != nil {
			opts.codec = codec
		}
	})
}

// Connection options.

// WithTLSConfig configures TLS for Redis connections.
func WithTLSConfig(cfg *tls.Config) Option {
	return optionFunc(func(opts *options) {
		if cfg != nil {
			opts.tls = cfg
		}
	})
}

// WithLimiter configures go-redis limiter for standalone and ring clients.
func WithLimiter(limiter rdb.Limiter) Option {
	return optionFunc(func(opts *options) {
		if limiter != nil {
			opts.limiter = limiter
		}
	})
}

// WithDialer configures custom Redis connection dialer.
func WithDialer(dialer func(ctx context.Context, network, addr string) (net.Conn, error)) Option {
	return optionFunc(func(opts *options) {
		if dialer != nil {
			opts.dialer = dialer
		}
	})
}

// WithOnConnect configures hook called when a Redis connection is established.
func WithOnConnect(fn func(ctx context.Context, cn *rdb.Conn) error) Option {
	return optionFunc(func(opts *options) {
		if fn != nil {
			opts.onConnect = fn
		}
	})
}

// WithDialerRetryBackoff configures dial retry backoff function.
func WithDialerRetryBackoff(fn func(attempt int) time.Duration) Option {
	return optionFunc(func(opts *options) {
		if fn != nil {
			opts.dialerRetryBackoff = fn
		}
	})
}

// Credentials options.

// WithCredentialsProvider configures Redis credentials provider.
func WithCredentialsProvider(provider func() (username, password string)) Option {
	return optionFunc(func(opts *options) {
		if provider != nil {
			opts.credentials.provider = provider
		}
	})
}

// WithCredentialsProviderContext configures context-aware Redis credentials provider.
func WithCredentialsProviderContext(
	provider func(ctx context.Context) (username, password string, err error),
) Option {
	return optionFunc(func(opts *options) {
		if provider != nil {
			opts.credentials.providerContext = provider
		}
	})
}

// WithStreamingCredentialsProvider configures streaming Redis credentials provider.
func WithStreamingCredentialsProvider(provider auth.StreamingCredentialsProvider) Option {
	return optionFunc(func(opts *options) {
		if provider != nil {
			opts.credentials.streamingProvider = provider
		}
	})
}

// Cluster options.

// WithClusterNewClient configures custom Redis Cluster node client factory.
func WithClusterNewClient(fn func(opt *rdb.Options) *rdb.Client) Option {
	return optionFunc(func(opts *options) {
		if fn != nil {
			opts.clusterNewClient = fn
		}
	})
}

// WithClusterSlots configures custom Redis Cluster slots discovery.
func WithClusterSlots(fn func(context.Context) ([]rdb.ClusterSlot, error)) Option {
	return optionFunc(func(opts *options) {
		if fn != nil {
			opts.clusterSlots = fn
		}
	})
}

// Push notification options.

// WithPushNotificationProcessor configures Redis push notification processor.
func WithPushNotificationProcessor(processor push.NotificationProcessor) Option {
	return optionFunc(func(opts *options) {
		if processor != nil {
			opts.pushNotificationProcessor = processor
		}
	})
}

// Ring options.

// WithRingNewClient configures custom Redis Ring shard client factory.
func WithRingNewClient(fn func(opt *rdb.Options) *rdb.Client) Option {
	return optionFunc(func(opts *options) {
		if fn != nil {
			opts.ringNewClient = fn
		}
	})
}

// WithRingHeartbeatFn configures Redis Ring shard health check function.
func WithRingHeartbeatFn(fn func(ctx context.Context, client *rdb.Client) bool) Option {
	return optionFunc(func(opts *options) {
		if fn != nil {
			opts.ringHeartbeatFn = fn
		}
	})
}

// WithRingConsistentHash configures Redis Ring consistent hash implementation.
func WithRingConsistentHash(fn func(shards []string) rdb.ConsistentHash) Option {
	return optionFunc(func(opts *options) {
		if fn != nil {
			opts.ringConsistentHash = fn
		}
	})
}

// Observability options.

// WithMetrics configures wrapper-level client metrics.
func WithMetrics(metrics Metrics) Option {
	return optionFunc(func(opts *options) {
		if metrics != nil {
			opts.metrics = metrics
		}
	})
}

// WithTracing configures client tracing instrumentation.
func WithTracing(tracing Tracing) Option {
	return optionFunc(func(opts *options) {
		if tracing != nil {
			opts.tracing = tracing
		}
	})
}
