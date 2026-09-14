package otelxredis

import (
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	metricNameCacheRequests           = "xredis.cache.requests"
	metricNameCacheLoaderDuration     = "xredis.cache.loader.duration"
	metricNameCacheSingleflightShared = "xredis.cache.singleflight.shared"
	metricNameLockOperations          = "xredis.lock.operations"
	metricNameRateLimiterDecisions    = "xredis.rate_limiter.decisions"
	metricNameRateLimiterDuration     = "xredis.rate_limiter.duration"
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

var cacheLoaderDurationBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10}

var rateLimiterDurationBuckets = []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5, 10}

func newCacheMetrics(meter metric.Meter, attributes metric.MeasurementOption) (cacheMetrics, error) {
	requests, err := meter.Int64Counter(
		metricNameCacheRequests,
		metric.WithDescription("Number of xredis cache requests."),
	)
	if err != nil {
		return cacheMetrics{}, err
	}

	loaderDuration, err := meter.Float64Histogram(
		metricNameCacheLoaderDuration,
		metric.WithDescription("Duration of xredis cache loader executions."),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(cacheLoaderDurationBuckets...),
	)
	if err != nil {
		return cacheMetrics{}, err
	}

	singleflightShared, err := meter.Int64Counter(
		metricNameCacheSingleflightShared,
		metric.WithDescription("Number of xredis cache requests that shared a singleflight result."),
	)
	if err != nil {
		return cacheMetrics{}, err
	}

	return cacheMetrics{
		requests:           requests,
		loaderDuration:     loaderDuration,
		singleflightShared: singleflightShared,
		attributes:         attributes,
	}, nil
}

func newLockMetrics(meter metric.Meter, attributes metric.MeasurementOption) (lockMetrics, error) {
	operations, err := meter.Int64Counter(
		metricNameLockOperations,
		metric.WithDescription("Number of xredis lock operations."),
	)
	if err != nil {
		return lockMetrics{}, err
	}

	return lockMetrics{
		operations: operations,
		attributes: attributes,
	}, nil
}

func newRateLimiterMetrics(meter metric.Meter, attributes metric.MeasurementOption) (rateLimiterMetrics, error) {
	decisions, err := meter.Int64Counter(
		metricNameRateLimiterDecisions,
		metric.WithDescription("Number of xredis rate limiter decisions."),
	)
	if err != nil {
		return rateLimiterMetrics{}, err
	}

	duration, err := meter.Float64Histogram(
		metricNameRateLimiterDuration,
		metric.WithDescription("Duration of xredis rate limiter decisions."),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(rateLimiterDurationBuckets...),
	)
	if err != nil {
		return rateLimiterMetrics{}, err
	}

	return rateLimiterMetrics{
		decisions:  decisions,
		duration:   duration,
		attributes: attributes,
	}, nil
}
