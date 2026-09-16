package otelxredis

import "go.opentelemetry.io/otel/metric"

const (
	metricNameCacheRequests           = "xredis.cache.requests"
	metricNameCacheLoaderDuration     = "xredis.cache.loader.duration"
	metricNameCacheSingleflightShared = "xredis.cache.singleflight.shared"
)

// cacheLoaderDurationBuckets defines explicit histogram boundaries, in seconds,
// for cache loader execution duration.
var cacheLoaderDurationBuckets = []float64{
	0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25,
	0.5, 0.75, 1, 2.5, 5, 7.5, 10,
}

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
		metric.WithDescription(
			"Number of xredis cache requests that shared a singleflight result.",
		),
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

const metricNameLockOperations = "xredis.lock.operations"

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

const (
	metricNameRateLimiterDecisions = "xredis.rate_limiter.decisions"
	metricNameRateLimiterDuration  = "xredis.rate_limiter.duration"
)

// rateLimiterDurationBuckets defines explicit histogram boundaries, in seconds,
// for rate limiter decision duration.
var rateLimiterDurationBuckets = []float64{
	0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5, 10,
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
