package otelxredis

import (
	"context"
	"sync"
	"time"

	"github.com/mkbeh/xredis"
	redisotelnative "github.com/redis/go-redis/extra/redisotel-native/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Metrics provides OpenTelemetry metrics for xredis wrapper-level operations
// and manages native go-redis metrics instrumentation.
//
// A Metrics instance is immutable after initialization and may be shared by
// multiple xredis clients.
type Metrics struct {
	cache       cacheMetrics
	lock        lockMetrics
	rateLimiter rateLimiterMetrics

	shutdown func() error
}

type cacheMetrics struct {
	requests           metric.Int64Counter
	loaderDuration     metric.Float64Histogram
	singleflightShared metric.Int64Counter
	attributes         metric.MeasurementOption
}

type lockMetrics struct {
	operations metric.Int64Counter
	attributes metric.MeasurementOption
}

type rateLimiterMetrics struct {
	decisions  metric.Int64Counter
	duration   metric.Float64Histogram
	attributes metric.MeasurementOption
}

var (
	_ xredis.Metrics            = (*Metrics)(nil)
	_ xredis.CacheMetrics       = (*cacheMetrics)(nil)
	_ xredis.LockMetrics        = (*lockMetrics)(nil)
	_ xredis.RateLimiterMetrics = (*rateLimiterMetrics)(nil)
)

// InitMetrics initializes xredis wrapper-level metrics and native go-redis
// metrics instrumentation.
//
// Call InitMetrics once during application startup before creating Redis
// clients. The returned Metrics may be shared by multiple clients.
func InitMetrics(opts ...MetricsOption) (*Metrics, error) {
	cfg := defaultMetricsConfig()
	for _, opt := range opts {
		if opt != nil {
			opt.apply(&cfg)
		}
	}

	provider := cfg.meterProvider
	if provider == nil {
		provider = otel.GetMeterProvider()
	}

	metrics, err := newMetrics(provider, &cfg)
	if err != nil {
		return nil, err
	}

	shutdownNative, err := initNativeMetrics(provider, &cfg)
	if err != nil {
		return nil, err
	}

	metrics.shutdown = sync.OnceValue(shutdownNative)

	return metrics, nil
}

// Register returns wrapper-level metrics configured for this Metrics instance.
func (m *Metrics) Register() xredis.ClientMetrics {
	if m == nil {
		return xredis.ClientMetrics{}
	}

	return xredis.ClientMetrics{
		Cache:       &m.cache,
		Lock:        &m.lock,
		RateLimiter: &m.rateLimiter,
	}
}

// Shutdown stops native go-redis metrics instrumentation.
//
// Shutdown is idempotent. It does not shut down the configured OpenTelemetry
// MeterProvider; the application remains responsible for that lifecycle.
func (m *Metrics) Shutdown() error {
	if m == nil || m.shutdown == nil {
		return nil
	}

	return m.shutdown()
}

func (m *cacheMetrics) RecordRequest(ctx context.Context, operation, result string) {
	m.requests.Add(
		ctx,
		1,
		m.attributes,
		metric.WithAttributes(
			attrCacheOperation.String(operation),
			attrCacheResult.String(result),
		),
	)
}

func (m *cacheMetrics) RecordLoaderDuration(ctx context.Context, outcome string, duration time.Duration) {
	m.loaderDuration.Record(
		ctx,
		duration.Seconds(),
		m.attributes,
		metric.WithAttributes(
			attrCacheLoaderOutcome.String(outcome),
		),
	)
}

func (m *cacheMetrics) RecordSingleflightShared(ctx context.Context) {
	m.singleflightShared.Add(ctx, 1, m.attributes)
}

func (m *lockMetrics) RecordOperation(ctx context.Context, lockType, operation, outcome string) {
	m.operations.Add(
		ctx,
		1,
		m.attributes,
		metric.WithAttributes(
			attrLockType.String(lockType),
			attrLockOperation.String(operation),
			attrLockOutcome.String(outcome),
		),
	)
}

func (m *rateLimiterMetrics) RecordDecision(ctx context.Context, algorithm, outcome string, duration time.Duration) {
	attributes := metric.WithAttributes(
		attrRateLimiterAlgorithm.String(algorithm),
		attrRateLimiterOutcome.String(outcome),
	)

	m.decisions.Add(ctx, 1, m.attributes, attributes)
	m.duration.Record(ctx, duration.Seconds(), m.attributes, attributes)
}

func newMetrics(provider metric.MeterProvider, cfg *metricsConfig) (*Metrics, error) {
	meter := provider.Meter(
		ScopeName,
		metric.WithInstrumentationVersion(Version()),
	)

	attributes := metric.WithAttributeSet(attributesFromConfig(cfg))

	cache, err := newCacheMetrics(meter, attributes)
	if err != nil {
		return nil, err
	}

	lock, err := newLockMetrics(meter, attributes)
	if err != nil {
		return nil, err
	}

	rateLimiter, err := newRateLimiterMetrics(meter, attributes)
	if err != nil {
		return nil, err
	}

	return &Metrics{
		cache:       cache,
		lock:        lock,
		rateLimiter: rateLimiter,
	}, nil
}

func attributesFromConfig(cfg *metricsConfig) attribute.Set {
	attrs := make([]attribute.KeyValue, 0, len(cfg.labels)+1)
	for key, value := range cfg.labels {
		attrs = append(attrs, attribute.String(key, value))
	}

	if cfg.clientID != "" {
		attrs = append(attrs, attrClientID.String(cfg.clientID))
	}

	return attribute.NewSet(attrs...)
}

func initNativeMetrics(
	provider metric.MeterProvider,
	cfg *metricsConfig,
) (func() error, error) {
	nativeCfg := redisotelnative.NewConfig().
		WithEnabled(true).
		WithMeterProvider(provider).
		WithMetricGroups(cfg.metricGroups).
		WithHidePubSubChannelNames(cfg.hidePubSubChannelNames).
		WithHideStreamNames(cfg.hideStreamNames)

	if len(cfg.includeCommands) > 0 {
		nativeCfg.WithIncludeCommands(cfg.includeCommands)
	}

	if len(cfg.excludeCommands) > 0 {
		nativeCfg.WithExcludeCommands(cfg.excludeCommands)
	}

	if cfg.histogramAggregationSet {
		nativeCfg.WithHistogramAggregation(cfg.histogramAggregation)
	}

	if len(cfg.histogramBuckets) > 0 {
		nativeCfg.WithHistogramBuckets(cfg.histogramBuckets)
	}

	instance := redisotelnative.GetObservabilityInstance()
	if err := instance.Init(nativeCfg); err != nil {
		return nil, err
	}

	return instance.Shutdown, nil
}
