package otelxredis

import (
	"context"
	"time"

	"github.com/mkbeh/xredis"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	attrClientID attribute.Key = "xredis.client.id"

	attrCacheOperation     attribute.Key = "xredis.cache.operation"
	attrCacheResult        attribute.Key = "xredis.cache.result"
	attrCacheLoaderOutcome attribute.Key = "xredis.cache.loader.outcome"

	attrLockType      attribute.Key = "xredis.lock.type"
	attrLockOperation attribute.Key = "xredis.lock.operation"
	attrLockOutcome   attribute.Key = "xredis.lock.outcome"

	attrRateLimiterAlgorithm attribute.Key = "xredis.rate_limiter.algorithm"
	attrRateLimiterOutcome   attribute.Key = "xredis.rate_limiter.outcome"
)

// Metrics provides OpenTelemetry metrics for xredis cache, lock, and rate
// limiter operations.
//
// A Metrics instance is immutable after initialization and may be shared by
// multiple xredis clients when the same static attributes should apply to all
// of them.
type Metrics struct {
	cache       cacheMetrics
	lock        lockMetrics
	rateLimiter rateLimiterMetrics
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

// NewMetrics creates a Metrics instance.
//
// If WithMeterProvider is not specified, NewMetrics uses the global
// OpenTelemetry MeterProvider.
func NewMetrics(opts ...MetricsOption) (*Metrics, error) {
	cfg := defaultMetricsConfig()
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	provider := cfg.meterProvider
	if provider == nil {
		provider = otel.GetMeterProvider()
	}

	return newMetrics(provider, &cfg)
}

// Register returns the cache, lock, and rate limiter metrics implementations
// provided by this Metrics instance.
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

func (m *cacheMetrics) RecordRequest(ctx context.Context, operation, result string) {
	attributes := metric.WithAttributeSet(attribute.NewSet(
		attrCacheOperation.String(operation),
		attrCacheResult.String(result),
	))

	m.requests.Add(ctx, 1, m.attributes, attributes)
}

func (m *cacheMetrics) RecordLoaderDuration(ctx context.Context, outcome string, duration time.Duration) {
	attributes := metric.WithAttributeSet(attribute.NewSet(
		attrCacheLoaderOutcome.String(outcome),
	))

	m.loaderDuration.Record(ctx, duration.Seconds(), m.attributes, attributes)
}

func (m *cacheMetrics) RecordSingleflightShared(ctx context.Context) {
	m.singleflightShared.Add(ctx, 1, m.attributes)
}

func (m *lockMetrics) RecordOperation(ctx context.Context, lockType, operation, outcome string) {
	attributes := metric.WithAttributeSet(attribute.NewSet(
		attrLockType.String(lockType),
		attrLockOperation.String(operation),
		attrLockOutcome.String(outcome),
	))

	m.operations.Add(ctx, 1, m.attributes, attributes)
}

func (m *rateLimiterMetrics) RecordDecision(ctx context.Context, algorithm, outcome string, duration time.Duration) {
	attributes := metric.WithAttributeSet(attribute.NewSet(
		attrRateLimiterAlgorithm.String(algorithm),
		attrRateLimiterOutcome.String(outcome),
	))

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
