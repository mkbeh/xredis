package xredis

import (
	"context"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const metricsInstrumentationName = "github.com/mkbeh/xredis"

// metrics contains wrapper-level metric instruments.
//
// The structure is initialized by InitObservability and is immutable after
// publication.
type metrics struct {
	attributes attribute.Set

	// Cache metrics.
	cacheRequests           metric.Int64Counter
	cacheLoaderDuration     metric.Float64Histogram
	cacheSingleflightShared metric.Int64Counter

	// Lock metrics.
	lockOperations metric.Int64Counter

	// Rate limiter metrics.
	rateLimitDecisions metric.Int64Counter
	rateLimitDuration  metric.Float64Histogram
}

var globalMetrics atomic.Pointer[metrics]

func newMetrics(provider metric.MeterProvider) (*metrics, error) {
	meter := provider.Meter(metricsInstrumentationName)

	cacheRequests, err := meter.Int64Counter(
		"redis.client.cache.requests",
		metric.WithDescription(
			"Number of Redis cache requests.",
		),
	)
	if err != nil {
		return nil, err
	}

	cacheLoaderDuration, err := meter.Float64Histogram(
		"redis.client.cache.loader.duration",
		metric.WithDescription(
			"Duration of Redis cache loader executions.",
		),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(
			cacheLoaderDurationBuckets...,
		),
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
		metric.WithDescription(
			"Number of Redis lock operations.",
		),
	)
	if err != nil {
		return nil, err
	}

	rateLimitDecisions, err := meter.Int64Counter(
		"redis.client.rate_limiter.decisions",
		metric.WithDescription(
			"Number of Redis rate limiter decisions.",
		),
	)
	if err != nil {
		return nil, err
	}

	rateLimitDuration, err := meter.Float64Histogram(
		"redis.client.rate_limiter.duration",
		metric.WithDescription(
			"Duration of Redis rate limiter decisions.",
		),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(
			rateLimitDurationBuckets...,
		),
	)
	if err != nil {
		return nil, err
	}

	return &metrics{
		cacheRequests:           cacheRequests,
		cacheLoaderDuration:     cacheLoaderDuration,
		cacheSingleflightShared: cacheSingleflightShared,
		lockOperations:          lockOperations,
		rateLimitDecisions:      rateLimitDecisions,
		rateLimitDuration:       rateLimitDuration,
	}, nil
}

func (m *metrics) recordCacheRequest(
	ctx context.Context,
	operation string,
	result string,
) {
	if m == nil {
		return
	}

	m.cacheRequests.Add(
		ctx,
		1,
		metric.WithAttributeSet(m.attributes),
		metric.WithAttributes(
			attribute.String(metricAttrCacheOperation, operation),
			attribute.String(metricAttrCacheResult, result),
		),
	)
}

func (m *metrics) recordCacheLoaderDuration(
	ctx context.Context,
	outcome string,
	duration time.Duration,
) {
	if m == nil {
		return
	}

	m.cacheLoaderDuration.Record(
		ctx,
		duration.Seconds(),
		metric.WithAttributeSet(m.attributes),
		metric.WithAttributes(
			attribute.String(metricAttrLoaderOutcome, outcome),
		),
	)
}

func (m *metrics) recordCacheSingleflightShared(ctx context.Context) {
	if m == nil {
		return
	}

	m.cacheSingleflightShared.Add(
		ctx,
		1,
		metric.WithAttributeSet(m.attributes),
	)
}

func (m *metrics) recordLockOperation(
	ctx context.Context,
	lockType string,
	operation string,
	outcome string,
) {
	if m == nil {
		return
	}

	m.lockOperations.Add(
		ctx,
		1,
		metric.WithAttributeSet(m.attributes),
		metric.WithAttributes(
			attribute.String(metricAttrLockType, lockType),
			attribute.String(metricAttrLockOperation, operation),
			attribute.String(metricAttrLockOutcome, outcome),
		),
	)
}

func (m *metrics) recordRateLimitDecision(
	ctx context.Context,
	algorithm string,
	outcome string,
	duration time.Duration,
) {
	if m == nil {
		return
	}

	options := []metric.RecordOption{
		metric.WithAttributeSet(m.attributes),
		metric.WithAttributes(
			attribute.String(metricAttrRateLimitAlgorithm, algorithm),
			attribute.String(metricAttrRateLimitOutcome, outcome),
		),
	}

	m.rateLimitDecisions.Add(
		ctx,
		1,
		metric.WithAttributeSet(m.attributes),
		metric.WithAttributes(
			attribute.String(metricAttrRateLimitAlgorithm, algorithm),
			attribute.String(metricAttrRateLimitOutcome, outcome),
		),
	)

	m.rateLimitDuration.Record(
		ctx,
		duration.Seconds(),
		options...,
	)
}

func newClientMetrics(labels map[string]string) *metrics {
	base := globalMetrics.Load()
	if base == nil {
		return nil
	}

	// Instruments are shared, while attributes belong to one Client.
	clientMetrics := *base
	clientMetrics.attributes = newMetricAttributes(labels)

	return &clientMetrics
}

func newMetricAttributes(labels map[string]string) attribute.Set {
	attrs := make([]attribute.KeyValue, 0, len(labels))

	for key, value := range labels {
		attrs = append(attrs, attribute.String(key, value))
	}

	return attribute.NewSet(attrs...)
}

func setMetrics(value *metrics) {
	globalMetrics.Store(value)
}

func clearMetrics(value *metrics) {
	globalMetrics.CompareAndSwap(value, nil)
}
