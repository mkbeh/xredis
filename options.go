package xredis

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/extra/redisotel/v9"
	rdb "github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/auth"
	"github.com/redis/go-redis/v9/maintnotifications"
	"github.com/redis/go-redis/v9/push"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// MarshallerFunc encodes values before storing them in Redis.
type MarshallerFunc func(value any) ([]byte, error)

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
	clientID       string
	identitySuffix string

	// Runtime dependencies.
	tls         *tls.Config
	logger      *slog.Logger
	limiter     rdb.Limiter
	marshaller  MarshallerFunc
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

	// Push and maintenance notifications.
	pushNotificationProcessor push.NotificationProcessor
	maintNotificationsConfig  *maintnotifications.Config

	// Tracing.
	traceOptions []redisotel.TracingOption
}

type credentialsOptions struct {
	provider          func() (username, password string)
	providerContext   func(ctx context.Context) (username, password string, err error)
	streamingProvider auth.StreamingCredentialsProvider
}

func newOptions(opts ...Option) *options {
	options := &options{
		logger:     slog.Default(),
		marshaller: json.Marshal,
	}

	for _, opt := range opts {
		if opt != nil {
			opt.apply(options)
		}
	}

	if options.clientID == "" {
		options.clientID = uuid.NewString()
	}

	if options.logger == nil {
		options.logger = slog.Default()
	}

	if options.marshaller == nil {
		options.marshaller = json.Marshal
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

func (o *options) addTraceOption(opt redisotel.TracingOption) {
	o.traceOptions = append(o.traceOptions, opt)
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

// Identity and logging options.

// WithClientID configures Redis client name and logging client_id.
func WithClientID(id string) Option {
	return optionFunc(func(opts *options) {
		if id != "" {
			opts.clientID = id
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

// WithLogger configures logger used by xredis.
func WithLogger(logger *slog.Logger) Option {
	return optionFunc(func(opts *options) {
		if logger != nil {
			opts.logger = logger
		}
	})
}

// Encoding options.

// WithMarshaller configures value marshaller used by xredis.
func WithMarshaller(fn MarshallerFunc) Option {
	return optionFunc(func(opts *options) {
		if fn != nil {
			opts.marshaller = fn
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

// Push and maintenance notification options.

// WithPushNotificationProcessor configures Redis push notification processor.
func WithPushNotificationProcessor(processor push.NotificationProcessor) Option {
	return optionFunc(func(opts *options) {
		if processor != nil {
			opts.pushNotificationProcessor = processor
		}
	})
}

// WithMaintNotificationsConfig configures Redis maintenance notifications.
func WithMaintNotificationsConfig(cfg *maintnotifications.Config) Option {
	return optionFunc(func(opts *options) {
		if cfg != nil {
			opts.maintNotificationsConfig = cfg
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

// Tracing options.

// WithTracerProvider enables tracing and configures OpenTelemetry tracer provider.
func WithTracerProvider(provider trace.TracerProvider) Option {
	return optionFunc(func(opts *options) {
		if provider != nil {
			opts.addTraceOption(redisotel.WithTracerProvider(provider))
		}
	})
}

// WithTracingDBStatement controls whether raw Redis commands are recorded in spans.
func WithTracingDBStatement(on bool) Option {
	return optionFunc(func(opts *options) {
		opts.addTraceOption(redisotel.WithDBStatement(on))
	})
}

// WithTracingDBSystem configures db.system attribute for tracing.
func WithTracingDBSystem(system string) Option {
	return optionFunc(func(opts *options) {
		if system != "" {
			opts.addTraceOption(redisotel.WithDBSystem(system))
		}
	})
}

// WithTracingAttributes configures additional OpenTelemetry tracing attributes.
func WithTracingAttributes(attrs ...attribute.KeyValue) Option {
	return optionFunc(func(opts *options) {
		if len(attrs) > 0 {
			opts.addTraceOption(redisotel.WithAttributes(attrs...))
		}
	})
}

// WithTracingCommandFilter configures command filtering for Redis tracing.
func WithTracingCommandFilter(filter func(cmd rdb.Cmder) bool) Option {
	return optionFunc(func(opts *options) {
		if filter != nil {
			opts.addTraceOption(redisotel.WithCommandFilter(filter))
		}
	})
}

// WithTracingCommandsFilter configures pipeline command filtering for Redis tracing.
func WithTracingCommandsFilter(filter func(cmds []rdb.Cmder) bool) Option {
	return optionFunc(func(opts *options) {
		if filter != nil {
			opts.addTraceOption(redisotel.WithCommandsFilter(filter))
		}
	})
}

// WithTracingDialFilter enables or disables filtering of dial commands in tracing.
func WithTracingDialFilter(on bool) Option {
	return optionFunc(func(opts *options) {
		opts.addTraceOption(redisotel.WithDialFilter(on))
	})
}

// WithTracingCallerEnabled controls whether tracing records caller file and line.
func WithTracingCallerEnabled(on bool) Option {
	return optionFunc(func(opts *options) {
		opts.addTraceOption(redisotel.WithCallerEnabled(on))
	})
}
