package otelxredis

import (
	"context"
	"sync"
	"time"

	"github.com/mkbeh/xredis"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const instrumentationName = "github.com/mkbeh/xredis"

const (
	attrCacheOperation = "redis.client.cache.operation"
	attrCacheResult    = "redis.client.cache.result"
	attrLoaderOutcome  = "redis.client.cache.loader.outcome"

	attrLockType      = "redis.client.lock.type"
	attrLockOperation = "redis.client.lock.operation"
	attrLockOutcome   = "redis.client.lock.outcome"

	attrRateLimitAlgorithm = "redis.client.rate_limiter.algorithm"
	attrRateLimitOutcome   = "redis.client.rate_limiter.outcome"
)

var cacheLoaderDurationBuckets = []float64{
	0.005,
	0.01,
	0.025,
	0.05,
	0.075,
	0.1,
	0.25,
	0.5,
	0.75,
	1,
	2.5,
	5,
	7.5,
	10,
}

var rateLimitDurationBuckets = []float64{
	0.0001,
	0.00025,
	0.0005,
	0.001,
	0.0025,
	0.005,
	0.01,
	0.025,
	0.05,
	0.1,
	0.25,
	0.5,
	1,
}

// Metrics provides OpenTelemetry metrics for xredis wrapper-level operations
// and manages native go-redis metrics instrumentation.
//
// A Metrics instance may be shared by multiple xredis clients. Shutdown must be
// called once by the application after all clients stop using it.
type Metrics struct {
	cacheRequests           metric.Int64Counter
	cacheLoaderDuration     metric.Float64Histogram
	cacheSingleflightShared metric.Int64Counter
	lockOperations          metric.Int64Counter
	rateLimitDecisions      metric.Int64Counter
	rateLimitDuration       metric.Float64Histogram

	shutdownNative func() error
	shutdownOnce   sync.Once
	shutdownErr    error
}

var _ xredis.Metrics = (*Metrics)(nil)

// Register creates wrapper-level metrics bound to client.
func (m *Metrics) Register(client *xredis.Client) xredis.ClientMetrics {
	if m == nil {
		return xredis.ClientMetrics{}
	}

	attributes := newAttributes(client.Labels())

	return xredis.ClientMetrics{
		Cache: &cacheMetrics{
			metrics:    m,
			attributes: attributes,
		},
		Lock: &lockMetrics{
			metrics:    m,
			attributes: attributes,
		},
		RateLimiter: &rateLimiterMetrics{
			metrics:    m,
			attributes: attributes,
		},
	}
}

// Shutdown stops native go-redis metrics instrumentation.
//
// Shutdown is idempotent. It does not shut down the configured OpenTelemetry
// MeterProvider; the application remains responsible for that lifecycle.
func (m *Metrics) Shutdown() error {
	if m == nil {
		return nil
	}

	m.shutdownOnce.Do(func() {
		if m.shutdownNative != nil {
			m.shutdownErr = m.shutdownNative()
		}
	})

	return m.shutdownErr
}

type cacheMetrics struct {
	metrics    *Metrics
	attributes attribute.Set
}

var _ xredis.CacheMetrics = (*cacheMetrics)(nil)

func (m *cacheMetrics) RecordRequest(
	ctx context.Context,
	operation,
	result string,
) {
	m.metrics.cacheRequests.Add(
		ctx,
		1,
		metric.WithAttributeSet(m.attributes),
		metric.WithAttributes(
			attribute.String(attrCacheOperation, operation),
			attribute.String(attrCacheResult, result),
		),
	)
}

func (m *cacheMetrics) RecordLoaderDuration(
	ctx context.Context,
	outcome string,
	duration time.Duration,
) {
	m.metrics.cacheLoaderDuration.Record(
		ctx,
		duration.Seconds(),
		metric.WithAttributeSet(m.attributes),
		metric.WithAttributes(
			attribute.String(attrLoaderOutcome, outcome),
		),
	)
}

func (m *cacheMetrics) RecordSingleflightShared(ctx context.Context) {
	m.metrics.cacheSingleflightShared.Add(
		ctx,
		1,
		metric.WithAttributeSet(m.attributes),
	)
}

type lockMetrics struct {
	metrics    *Metrics
	attributes attribute.Set
}

var _ xredis.LockMetrics = (*lockMetrics)(nil)

func (m *lockMetrics) RecordOperation(
	ctx context.Context,
	lockType,
	operation,
	outcome string,
) {
	m.metrics.lockOperations.Add(
		ctx,
		1,
		metric.WithAttributeSet(m.attributes),
		metric.WithAttributes(
			attribute.String(attrLockType, lockType),
			attribute.String(attrLockOperation, operation),
			attribute.String(attrLockOutcome, outcome),
		),
	)
}

type rateLimiterMetrics struct {
	metrics    *Metrics
	attributes attribute.Set
}

var _ xredis.RateLimiterMetrics = (*rateLimiterMetrics)(nil)

func (m *rateLimiterMetrics) RecordDecision(
	ctx context.Context,
	algorithm,
	outcome string,
	duration time.Duration,
) {
	options := []metric.RecordOption{
		metric.WithAttributeSet(m.attributes),
		metric.WithAttributes(
			attribute.String(attrRateLimitAlgorithm, algorithm),
			attribute.String(attrRateLimitOutcome, outcome),
		),
	}

	m.metrics.rateLimitDecisions.Add(
		ctx,
		1,
		metric.WithAttributeSet(m.attributes),
		metric.WithAttributes(
			attribute.String(attrRateLimitAlgorithm, algorithm),
			attribute.String(attrRateLimitOutcome, outcome),
		),
	)

	m.metrics.rateLimitDuration.Record(
		ctx,
		duration.Seconds(),
		options...,
	)
}

func newMetrics(provider metric.MeterProvider) (*Metrics, error) {
	meter := provider.Meter(instrumentationName)

	cacheRequests, err := meter.Int64Counter(
		"redis.client.cache.requests",
		metric.WithDescription("Number of Redis cache requests."),
	)
	if err != nil {
		return nil, err
	}

	cacheLoaderDuration, err := meter.Float64Histogram(
		"redis.client.cache.loader.duration",
		metric.WithDescription("Duration of Redis cache loader executions."),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(cacheLoaderDurationBuckets...),
	)
	if err != nil {
		return nil, err
	}

	cacheSingleflightShared, err := meter.Int64Counter(
		"redis.client.cache.singleflight.shared",
		metric.WithDescription(
			"Number of Redis cache requests that shared a singleflight result.",
		),
	)
	if err != nil {
		return nil, err
	}

	lockOperations, err := meter.Int64Counter(
		"redis.client.lock.operations",
		metric.WithDescription("Number of Redis lock operations."),
	)
	if err != nil {
		return nil, err
	}

	rateLimitDecisions, err := meter.Int64Counter(
		"redis.client.rate_limiter.decisions",
		metric.WithDescription("Number of Redis rate limiter decisions."),
	)
	if err != nil {
		return nil, err
	}

	rateLimitDuration, err := meter.Float64Histogram(
		"redis.client.rate_limiter.duration",
		metric.WithDescription("Duration of Redis rate limiter decisions."),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(rateLimitDurationBuckets...),
	)
	if err != nil {
		return nil, err
	}

	return &Metrics{
		cacheRequests:           cacheRequests,
		cacheLoaderDuration:     cacheLoaderDuration,
		cacheSingleflightShared: cacheSingleflightShared,
		lockOperations:          lockOperations,
		rateLimitDecisions:      rateLimitDecisions,
		rateLimitDuration:       rateLimitDuration,
	}, nil
}

func newAttributes(labels map[string]string) attribute.Set {
	attrs := make([]attribute.KeyValue, 0, len(labels))

	for key, value := range labels {
		attrs = append(attrs, attribute.String(key, value))
	}

	return attribute.NewSet(attrs...)
}
