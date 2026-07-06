package xredis

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"time"

	rdb "github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/auth"
)

// Configuration types.

// ClientConfig configures a standalone Redis client.
type ClientConfig struct {
	// URL contains Redis connection URL.
	// If set, it takes precedence over other connection fields.
	URL string

	// Network defines the network type: tcp or unix.
	Network string

	// Addr contains Redis address.
	Addr string

	// NodeAddress is the Redis node address as reported by the server.
	NodeAddress string

	// Protocol defines RESP protocol version.
	Protocol int

	// Username is used for Redis ACL authentication.
	Username string

	// Password is used for Redis authentication.
	Password string

	// DB defines Redis database.
	DB int

	// MaxRetries defines the maximum number of command retries.
	MaxRetries int

	// MinRetryBackoff defines the minimum backoff between retries.
	MinRetryBackoff time.Duration

	// MaxRetryBackoff defines the maximum backoff between retries.
	MaxRetryBackoff time.Duration

	// DialTimeout defines timeout for establishing new connections.
	DialTimeout time.Duration

	// DialerRetries defines the maximum number of retry attempts for dialing.
	DialerRetries int

	// DialerRetryTimeout defines backoff duration between dial retry attempts.
	DialerRetryTimeout time.Duration

	// ReadTimeout defines socket read timeout.
	ReadTimeout time.Duration

	// WriteTimeout defines socket write timeout.
	WriteTimeout time.Duration

	// ContextTimeoutEnabled controls whether client respects context timeouts and deadlines.
	ContextTimeoutEnabled bool

	// ReadBufferSize defines Redis read buffer size per connection.
	ReadBufferSize int

	// WriteBufferSize defines Redis write buffer size per connection.
	WriteBufferSize int

	// PoolFIFO enables FIFO pool mode instead of default LIFO mode.
	PoolFIFO bool

	// PoolSize defines base connection pool size.
	PoolSize int

	// MaxConcurrentDials limits concurrent connection creation.
	MaxConcurrentDials int

	// PoolTimeout defines how long client waits for a free connection.
	PoolTimeout time.Duration

	// MinIdleConns defines minimum number of idle connections.
	MinIdleConns int

	// MaxIdleConns defines maximum number of idle connections.
	MaxIdleConns int

	// MaxActiveConns limits allocated connections.
	MaxActiveConns int

	// ConnMaxIdleTime defines maximum connection idle time.
	ConnMaxIdleTime time.Duration

	// ConnMaxLifetime defines maximum connection lifetime.
	ConnMaxLifetime time.Duration

	// ConnMaxLifetimeJitter adds jitter to connection lifetime.
	ConnMaxLifetimeJitter time.Duration

	// DisableIdentity disables Redis client identity command on connect.
	DisableIdentity bool

	// IdentitySuffix adds suffix to go-redis client identity.
	IdentitySuffix string

	// FailingTimeoutSeconds defines how long node is avoided after failure.
	FailingTimeoutSeconds int
}

// ClusterConfig configures a Redis Cluster client.
type ClusterConfig struct {
	// URL contains Redis Cluster connection URL.
	// If set, it takes precedence over other connection fields.
	URL string

	// Addrs contains Redis Cluster seed node addresses.
	Addrs []string

	// Protocol defines RESP protocol version.
	Protocol int

	// Username is used for Redis ACL authentication.
	Username string

	// Password is used for Redis authentication.
	Password string

	// MaxRedirects defines the maximum number of cluster redirects.
	MaxRedirects int

	// ReadOnly enables read-only commands on replica nodes.
	ReadOnly bool

	// RouteByLatency routes read-only commands to the closest node.
	RouteByLatency bool

	// RouteRandomly routes read-only commands to a random node.
	RouteRandomly bool

	// MaxRetries defines the maximum number of command retries.
	MaxRetries int

	// MinRetryBackoff defines the minimum backoff between retries.
	MinRetryBackoff time.Duration

	// MaxRetryBackoff defines the maximum backoff between retries.
	MaxRetryBackoff time.Duration

	// DialTimeout defines timeout for establishing new connections.
	DialTimeout time.Duration

	// DialerRetries defines the maximum number of retry attempts for dialing.
	DialerRetries int

	// DialerRetryTimeout defines backoff duration between dial retry attempts.
	DialerRetryTimeout time.Duration

	// ReadTimeout defines socket read timeout.
	ReadTimeout time.Duration

	// WriteTimeout defines socket write timeout.
	WriteTimeout time.Duration

	// ContextTimeoutEnabled controls whether client respects context timeouts and deadlines.
	ContextTimeoutEnabled bool

	// ReadBufferSize defines Redis read buffer size per connection.
	ReadBufferSize int

	// WriteBufferSize defines Redis write buffer size per connection.
	WriteBufferSize int

	// PoolFIFO enables FIFO pool mode instead of default LIFO mode.
	PoolFIFO bool

	// PoolSize defines base connection pool size per cluster node.
	PoolSize int

	// MaxConcurrentDials limits concurrent connection creation.
	MaxConcurrentDials int

	// PoolTimeout defines how long client waits for a free connection.
	PoolTimeout time.Duration

	// MinIdleConns defines minimum number of idle connections.
	MinIdleConns int

	// MaxIdleConns defines maximum number of idle connections.
	MaxIdleConns int

	// MaxActiveConns limits allocated connections per cluster node.
	MaxActiveConns int

	// ConnMaxIdleTime defines maximum connection idle time.
	ConnMaxIdleTime time.Duration

	// ConnMaxLifetime defines maximum connection lifetime.
	ConnMaxLifetime time.Duration

	// ConnMaxLifetimeJitter adds jitter to connection lifetime.
	ConnMaxLifetimeJitter time.Duration

	// DisableIdentity disables Redis client identity command on connect.
	DisableIdentity bool

	// IdentitySuffix adds suffix to go-redis client identity.
	IdentitySuffix string

	// FailingTimeoutSeconds defines how long cluster node is avoided after failure.
	FailingTimeoutSeconds int

	// DisableRoutingPolicies disables experimental cluster routing policies.
	DisableRoutingPolicies bool

	// ClusterStateReloadInterval defines how often cluster state is refreshed.
	ClusterStateReloadInterval time.Duration
}

// FailoverConfig configures a Redis Sentinel / failover client.
type FailoverConfig struct {
	// URL contains Redis Sentinel/failover connection URL.
	// If set, it takes precedence over other connection fields.
	URL string

	// MasterName defines Redis Sentinel master name.
	MasterName string

	// SentinelAddrs contains Redis Sentinel node addresses.
	SentinelAddrs []string

	// SentinelUsername is used for Sentinel authentication.
	SentinelUsername string

	// SentinelPassword is used for Sentinel authentication.
	SentinelPassword string

	// Protocol defines RESP protocol version.
	Protocol int

	// Username is used for Redis ACL authentication.
	Username string

	// Password is used for Redis authentication.
	Password string

	// DB defines Redis database.
	DB int

	// ReplicaOnly routes all commands to read-only replicas.
	ReplicaOnly bool

	// UseDisconnectedReplicas allows reads from disconnected replicas.
	UseDisconnectedReplicas bool

	// RouteByLatency routes read-only commands to the closest node.
	RouteByLatency bool

	// RouteRandomly routes read-only commands to a random node.
	RouteRandomly bool

	// MaxRetries defines the maximum number of command retries.
	MaxRetries int

	// MinRetryBackoff defines the minimum backoff between retries.
	MinRetryBackoff time.Duration

	// MaxRetryBackoff defines the maximum backoff between retries.
	MaxRetryBackoff time.Duration

	// DialTimeout defines timeout for establishing new connections.
	DialTimeout time.Duration

	// DialerRetries defines the maximum number of retry attempts for dialing.
	DialerRetries int

	// DialerRetryTimeout defines backoff duration between dial retry attempts.
	DialerRetryTimeout time.Duration

	// ReadTimeout defines socket read timeout.
	ReadTimeout time.Duration

	// WriteTimeout defines socket write timeout.
	WriteTimeout time.Duration

	// ContextTimeoutEnabled controls whether client respects context timeouts and deadlines.
	ContextTimeoutEnabled bool

	// ReadBufferSize defines Redis read buffer size per connection.
	ReadBufferSize int

	// WriteBufferSize defines Redis write buffer size per connection.
	WriteBufferSize int

	// PoolFIFO enables FIFO pool mode instead of default LIFO mode.
	PoolFIFO bool

	// PoolSize defines base connection pool size.
	PoolSize int

	// MaxConcurrentDials limits concurrent connection creation.
	MaxConcurrentDials int

	// PoolTimeout defines how long client waits for a free connection.
	PoolTimeout time.Duration

	// MinIdleConns defines minimum number of idle connections.
	MinIdleConns int

	// MaxIdleConns defines maximum number of idle connections.
	MaxIdleConns int

	// MaxActiveConns limits allocated connections.
	MaxActiveConns int

	// ConnMaxIdleTime defines maximum connection idle time.
	ConnMaxIdleTime time.Duration

	// ConnMaxLifetime defines maximum connection lifetime.
	ConnMaxLifetime time.Duration

	// ConnMaxLifetimeJitter adds jitter to connection lifetime.
	ConnMaxLifetimeJitter time.Duration

	// DisableIdentity disables Redis client identity command on connect.
	DisableIdentity bool

	// IdentitySuffix adds suffix to go-redis client identity.
	IdentitySuffix string

	// FailingTimeoutSeconds defines how long node is avoided after failure.
	FailingTimeoutSeconds int
}

// RingConfig configures a Redis Ring client for client-side sharding.
type RingConfig struct {
	// Addrs contains named Redis shards.
	Addrs map[string]string

	// HeartbeatFrequency defines how often ring shards are checked for availability.
	HeartbeatFrequency time.Duration

	// Protocol defines RESP protocol version.
	Protocol int

	// Username is used for Redis ACL authentication.
	Username string

	// Password is used for Redis authentication.
	Password string

	// DB defines Redis database.
	DB int

	// MaxRetries defines the maximum number of command retries.
	MaxRetries int

	// MinRetryBackoff defines the minimum backoff between retries.
	MinRetryBackoff time.Duration

	// MaxRetryBackoff defines the maximum backoff between retries.
	MaxRetryBackoff time.Duration

	// DialTimeout defines timeout for establishing new connections.
	DialTimeout time.Duration

	// DialerRetries defines the maximum number of retry attempts for dialing.
	DialerRetries int

	// DialerRetryTimeout defines backoff duration between dial retry attempts.
	DialerRetryTimeout time.Duration

	// ReadTimeout defines socket read timeout.
	ReadTimeout time.Duration

	// WriteTimeout defines socket write timeout.
	WriteTimeout time.Duration

	// ContextTimeoutEnabled controls whether client respects context timeouts and deadlines.
	ContextTimeoutEnabled bool

	// ReadBufferSize defines Redis read buffer size per connection.
	ReadBufferSize int

	// WriteBufferSize defines Redis write buffer size per connection.
	WriteBufferSize int

	// PoolFIFO enables FIFO pool mode instead of default LIFO mode.
	PoolFIFO bool

	// PoolSize defines base connection pool size.
	PoolSize int

	// PoolTimeout defines how long client waits for a free connection.
	PoolTimeout time.Duration

	// MinIdleConns defines minimum number of idle connections.
	MinIdleConns int

	// MaxIdleConns defines maximum number of idle connections.
	MaxIdleConns int

	// MaxActiveConns limits allocated connections.
	MaxActiveConns int

	// ConnMaxIdleTime defines maximum connection idle time.
	ConnMaxIdleTime time.Duration

	// ConnMaxLifetime defines maximum connection lifetime.
	ConnMaxLifetime time.Duration

	// ConnMaxLifetimeJitter adds jitter to connection lifetime.
	ConnMaxLifetimeJitter time.Duration

	// DisableIdentity disables Redis client identity command on connect.
	DisableIdentity bool

	// IdentitySuffix adds suffix to go-redis client identity.
	IdentitySuffix string
}

// Config parsing.

func parseClientConfig(cfg *ClientConfig) (*rdb.Options, error) {
	if strings.TrimSpace(cfg.URL) != "" {
		return rdb.ParseURL(cfg.URL)
	}

	redisOpts := &rdb.Options{
		Network:               cfg.Network,
		Addr:                  cfg.Addr,
		NodeAddress:           cfg.NodeAddress,
		Protocol:              cfg.Protocol,
		Username:              cfg.Username,
		Password:              cfg.Password,
		DB:                    cfg.DB,
		MaxRetries:            cfg.MaxRetries,
		MinRetryBackoff:       cfg.MinRetryBackoff,
		MaxRetryBackoff:       cfg.MaxRetryBackoff,
		DialTimeout:           cfg.DialTimeout,
		DialerRetries:         cfg.DialerRetries,
		DialerRetryTimeout:    cfg.DialerRetryTimeout,
		ReadTimeout:           cfg.ReadTimeout,
		WriteTimeout:          cfg.WriteTimeout,
		ContextTimeoutEnabled: cfg.ContextTimeoutEnabled,
		ReadBufferSize:        cfg.ReadBufferSize,
		WriteBufferSize:       cfg.WriteBufferSize,
		PoolFIFO:              cfg.PoolFIFO,
		PoolSize:              cfg.PoolSize,
		MaxConcurrentDials:    cfg.MaxConcurrentDials,
		PoolTimeout:           cfg.PoolTimeout,
		MinIdleConns:          cfg.MinIdleConns,
		MaxIdleConns:          cfg.MaxIdleConns,
		MaxActiveConns:        cfg.MaxActiveConns,
		ConnMaxIdleTime:       cfg.ConnMaxIdleTime,
		ConnMaxLifetime:       cfg.ConnMaxLifetime,
		ConnMaxLifetimeJitter: cfg.ConnMaxLifetimeJitter,
		DisableIdentity:       cfg.DisableIdentity,
		IdentitySuffix:        cfg.IdentitySuffix,
		FailingTimeoutSeconds: cfg.FailingTimeoutSeconds,
	}

	return redisOpts, nil
}

func parseClusterConfig(cfg *ClusterConfig) (*rdb.ClusterOptions, error) {
	if strings.TrimSpace(cfg.URL) != "" {
		return rdb.ParseClusterURL(cfg.URL)
	}

	addrs := normalizeAddrs(cfg.Addrs)
	if len(addrs) == 0 {
		return nil, fmt.Errorf("%w: redis cluster addresses are required", ErrInvalidConfig)
	}

	redisOpts := &rdb.ClusterOptions{
		Addrs:                      addrs,
		Protocol:                   cfg.Protocol,
		Username:                   cfg.Username,
		Password:                   cfg.Password,
		MaxRedirects:               cfg.MaxRedirects,
		ReadOnly:                   cfg.ReadOnly,
		RouteByLatency:             cfg.RouteByLatency,
		RouteRandomly:              cfg.RouteRandomly,
		MaxRetries:                 cfg.MaxRetries,
		MinRetryBackoff:            cfg.MinRetryBackoff,
		MaxRetryBackoff:            cfg.MaxRetryBackoff,
		DialTimeout:                cfg.DialTimeout,
		DialerRetries:              cfg.DialerRetries,
		DialerRetryTimeout:         cfg.DialerRetryTimeout,
		ReadTimeout:                cfg.ReadTimeout,
		WriteTimeout:               cfg.WriteTimeout,
		ContextTimeoutEnabled:      cfg.ContextTimeoutEnabled,
		ReadBufferSize:             cfg.ReadBufferSize,
		WriteBufferSize:            cfg.WriteBufferSize,
		PoolFIFO:                   cfg.PoolFIFO,
		PoolSize:                   cfg.PoolSize,
		MaxConcurrentDials:         cfg.MaxConcurrentDials,
		PoolTimeout:                cfg.PoolTimeout,
		MinIdleConns:               cfg.MinIdleConns,
		MaxIdleConns:               cfg.MaxIdleConns,
		MaxActiveConns:             cfg.MaxActiveConns,
		ConnMaxIdleTime:            cfg.ConnMaxIdleTime,
		ConnMaxLifetime:            cfg.ConnMaxLifetime,
		ConnMaxLifetimeJitter:      cfg.ConnMaxLifetimeJitter,
		DisableIdentity:            cfg.DisableIdentity,
		IdentitySuffix:             cfg.IdentitySuffix,
		FailingTimeoutSeconds:      cfg.FailingTimeoutSeconds,
		DisableRoutingPolicies:     cfg.DisableRoutingPolicies,
		ClusterStateReloadInterval: cfg.ClusterStateReloadInterval,
	}

	return redisOpts, nil
}

func parseFailoverConfig(cfg *FailoverConfig) (*rdb.FailoverOptions, error) {
	if strings.TrimSpace(cfg.URL) != "" {
		return rdb.ParseFailoverURL(cfg.URL)
	}

	addrs := normalizeAddrs(cfg.SentinelAddrs)
	if len(addrs) == 0 {
		return nil, fmt.Errorf("%w: redis sentinel addresses are required", ErrInvalidConfig)
	}

	if strings.TrimSpace(cfg.MasterName) == "" {
		return nil, fmt.Errorf("%w: redis sentinel master name is required", ErrInvalidConfig)
	}

	redisOpts := &rdb.FailoverOptions{
		MasterName:              cfg.MasterName,
		SentinelAddrs:           addrs,
		SentinelUsername:        cfg.SentinelUsername,
		SentinelPassword:        cfg.SentinelPassword,
		Protocol:                cfg.Protocol,
		Username:                cfg.Username,
		Password:                cfg.Password,
		DB:                      cfg.DB,
		ReplicaOnly:             cfg.ReplicaOnly,
		UseDisconnectedReplicas: cfg.UseDisconnectedReplicas,
		RouteByLatency:          cfg.RouteByLatency,
		RouteRandomly:           cfg.RouteRandomly,
		MaxRetries:              cfg.MaxRetries,
		MinRetryBackoff:         cfg.MinRetryBackoff,
		MaxRetryBackoff:         cfg.MaxRetryBackoff,
		DialTimeout:             cfg.DialTimeout,
		DialerRetries:           cfg.DialerRetries,
		DialerRetryTimeout:      cfg.DialerRetryTimeout,
		ReadTimeout:             cfg.ReadTimeout,
		WriteTimeout:            cfg.WriteTimeout,
		ContextTimeoutEnabled:   cfg.ContextTimeoutEnabled,
		ReadBufferSize:          cfg.ReadBufferSize,
		WriteBufferSize:         cfg.WriteBufferSize,
		PoolFIFO:                cfg.PoolFIFO,
		PoolSize:                cfg.PoolSize,
		MaxConcurrentDials:      cfg.MaxConcurrentDials,
		PoolTimeout:             cfg.PoolTimeout,
		MinIdleConns:            cfg.MinIdleConns,
		MaxIdleConns:            cfg.MaxIdleConns,
		MaxActiveConns:          cfg.MaxActiveConns,
		ConnMaxIdleTime:         cfg.ConnMaxIdleTime,
		ConnMaxLifetime:         cfg.ConnMaxLifetime,
		ConnMaxLifetimeJitter:   cfg.ConnMaxLifetimeJitter,
		DisableIdentity:         cfg.DisableIdentity,
		IdentitySuffix:          cfg.IdentitySuffix,
		FailingTimeoutSeconds:   cfg.FailingTimeoutSeconds,
	}

	return redisOpts, nil
}

func parseRingConfig(cfg *RingConfig) (*rdb.RingOptions, error) {
	addrs := normalizeRingAddrs(cfg.Addrs)
	if len(addrs) == 0 {
		return nil, fmt.Errorf("%w: redis ring addresses are required", ErrInvalidConfig)
	}

	redisOpts := &rdb.RingOptions{
		Addrs:                 addrs,
		HeartbeatFrequency:    cfg.HeartbeatFrequency,
		Protocol:              cfg.Protocol,
		Username:              cfg.Username,
		Password:              cfg.Password,
		DB:                    cfg.DB,
		MaxRetries:            cfg.MaxRetries,
		MinRetryBackoff:       cfg.MinRetryBackoff,
		MaxRetryBackoff:       cfg.MaxRetryBackoff,
		DialTimeout:           cfg.DialTimeout,
		DialerRetries:         cfg.DialerRetries,
		DialerRetryTimeout:    cfg.DialerRetryTimeout,
		ReadTimeout:           cfg.ReadTimeout,
		WriteTimeout:          cfg.WriteTimeout,
		ContextTimeoutEnabled: cfg.ContextTimeoutEnabled,
		ReadBufferSize:        cfg.ReadBufferSize,
		WriteBufferSize:       cfg.WriteBufferSize,
		PoolFIFO:              cfg.PoolFIFO,
		PoolSize:              cfg.PoolSize,
		PoolTimeout:           cfg.PoolTimeout,
		MinIdleConns:          cfg.MinIdleConns,
		MaxIdleConns:          cfg.MaxIdleConns,
		MaxActiveConns:        cfg.MaxActiveConns,
		ConnMaxIdleTime:       cfg.ConnMaxIdleTime,
		ConnMaxLifetime:       cfg.ConnMaxLifetime,
		ConnMaxLifetimeJitter: cfg.ConnMaxLifetimeJitter,
		DisableIdentity:       cfg.DisableIdentity,
		IdentitySuffix:        cfg.IdentitySuffix,
	}

	return redisOpts, nil
}

// Runtime option application.

func applyClientOptions(redisOpts *rdb.Options, opts *options) {
	applyCommonOptions(
		&redisOpts.ClientName,
		&redisOpts.IdentitySuffix,
		&redisOpts.TLSConfig,
		opts,
	)

	applyConnectionOptions(
		&redisOpts.Dialer,
		&redisOpts.OnConnect,
		&redisOpts.DialerRetryBackoff,
		opts,
	)

	applyCredentials(
		&redisOpts.CredentialsProvider,
		&redisOpts.CredentialsProviderContext,
		&redisOpts.StreamingCredentialsProvider,
		opts.credentials,
	)

	if opts.limiter != nil {
		redisOpts.Limiter = opts.limiter
	}

	if opts.pushNotificationProcessor != nil {
		redisOpts.PushNotificationProcessor = opts.pushNotificationProcessor
	}

	if opts.maintNotificationsConfig != nil {
		redisOpts.MaintNotificationsConfig = opts.maintNotificationsConfig
	}
}

func applyClusterOptions(redisOpts *rdb.ClusterOptions, opts *options) {
	applyCommonOptions(
		&redisOpts.ClientName,
		&redisOpts.IdentitySuffix,
		&redisOpts.TLSConfig,
		opts,
	)

	applyConnectionOptions(
		&redisOpts.Dialer,
		&redisOpts.OnConnect,
		&redisOpts.DialerRetryBackoff,
		opts,
	)

	applyCredentials(
		&redisOpts.CredentialsProvider,
		&redisOpts.CredentialsProviderContext,
		&redisOpts.StreamingCredentialsProvider,
		opts.credentials,
	)

	if opts.clusterNewClient != nil {
		redisOpts.NewClient = opts.clusterNewClient
	}

	if opts.clusterSlots != nil {
		redisOpts.ClusterSlots = opts.clusterSlots
	}

	if opts.pushNotificationProcessor != nil {
		redisOpts.PushNotificationProcessor = opts.pushNotificationProcessor
	}

	if opts.maintNotificationsConfig != nil {
		redisOpts.MaintNotificationsConfig = opts.maintNotificationsConfig
	}
}

func applyFailoverOptions(redisOpts *rdb.FailoverOptions, opts *options) {
	applyCommonOptions(
		&redisOpts.ClientName,
		&redisOpts.IdentitySuffix,
		&redisOpts.TLSConfig,
		opts,
	)

	applyConnectionOptions(
		&redisOpts.Dialer,
		&redisOpts.OnConnect,
		&redisOpts.DialerRetryBackoff,
		opts,
	)

	applyCredentials(
		&redisOpts.CredentialsProvider,
		&redisOpts.CredentialsProviderContext,
		&redisOpts.StreamingCredentialsProvider,
		opts.credentials,
	)

	if opts.pushNotificationProcessor != nil {
		redisOpts.PushNotificationProcessor = opts.pushNotificationProcessor
	}
}

func applyRingOptions(redisOpts *rdb.RingOptions, opts *options) {
	applyCommonOptions(
		&redisOpts.ClientName,
		&redisOpts.IdentitySuffix,
		&redisOpts.TLSConfig,
		opts,
	)

	applyConnectionOptions(
		&redisOpts.Dialer,
		&redisOpts.OnConnect,
		&redisOpts.DialerRetryBackoff,
		opts,
	)

	applyCredentials(
		&redisOpts.CredentialsProvider,
		&redisOpts.CredentialsProviderContext,
		&redisOpts.StreamingCredentialsProvider,
		opts.credentials,
	)

	if opts.limiter != nil {
		redisOpts.Limiter = opts.limiter
	}

	if opts.ringNewClient != nil {
		redisOpts.NewClient = opts.ringNewClient
	}

	if opts.ringHeartbeatFn != nil {
		redisOpts.HeartbeatFn = opts.ringHeartbeatFn
	}

	if opts.ringConsistentHash != nil {
		redisOpts.NewConsistentHash = opts.ringConsistentHash
	}
}

// Shared option helpers.

func applyCommonOptions(
	clientName *string,
	identitySuffix *string,
	tlsConfigField **tls.Config,
	opts *options,
) {
	*clientName = opts.clientID

	if opts.identitySuffix != "" {
		*identitySuffix = opts.identitySuffix
	}

	if opts.tls != nil {
		*tlsConfigField = opts.tls
	}
}

func applyCredentials(
	provider *func() (username, password string),
	providerContext *func(ctx context.Context) (username, password string, err error),
	streamingProvider *auth.StreamingCredentialsProvider,
	credentials credentialsOptions,
) {
	if credentials.provider != nil {
		*provider = credentials.provider
	}

	if credentials.providerContext != nil {
		*providerContext = credentials.providerContext
	}

	if credentials.streamingProvider != nil {
		*streamingProvider = credentials.streamingProvider
	}
}

func applyConnectionOptions(
	dialer *func(ctx context.Context, network, addr string) (net.Conn, error),
	onConnect *func(ctx context.Context, cn *rdb.Conn) error,
	dialerRetryBackoff *func(attempt int) time.Duration,
	opts *options,
) {
	if opts.dialer != nil {
		*dialer = opts.dialer
	}

	if opts.onConnect != nil {
		*onConnect = opts.onConnect
	}

	if opts.dialerRetryBackoff != nil {
		*dialerRetryBackoff = opts.dialerRetryBackoff
	}
}

// Address helpers.

func normalizeAddrs(addrs []string) []string {
	if len(addrs) == 0 {
		return nil
	}

	out := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		addr = strings.TrimSpace(addr)
		if addr != "" {
			out = append(out, addr)
		}
	}

	return out
}

func normalizeRingAddrs(addrs map[string]string) map[string]string {
	if len(addrs) == 0 {
		return nil
	}

	out := make(map[string]string, len(addrs))
	for name, addr := range addrs {
		name = strings.TrimSpace(name)
		addr = strings.TrimSpace(addr)
		if name != "" && addr != "" {
			out[name] = addr
		}
	}

	return out
}
