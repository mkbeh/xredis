package otelxredis

import (
	redisotelnative "github.com/redis/go-redis/extra/redisotel-native/v9"
	"go.opentelemetry.io/otel/metric"
)

// RedisMetricGroupFlags defines redisotel-native metric groups.
type RedisMetricGroupFlags = redisotelnative.MetricGroupFlags

// RedisHistogramAggregation defines histogram aggregation mode for redisotel-native metrics.
type RedisHistogramAggregation = redisotelnative.HistogramAggregation

const (
	// RedisMetricGroupCommand enables Redis command metrics.
	RedisMetricGroupCommand = redisotelnative.MetricGroupFlagCommand

	// RedisMetricGroupConnectionBasic enables basic connection metrics.
	RedisMetricGroupConnectionBasic = redisotelnative.MetricGroupFlagConnectionBasic

	// RedisMetricGroupResiliency enables Redis resiliency metrics.
	RedisMetricGroupResiliency = redisotelnative.MetricGroupFlagResiliency

	// RedisMetricGroupConnectionAdvanced enables advanced connection metrics.
	RedisMetricGroupConnectionAdvanced = redisotelnative.MetricGroupFlagConnectionAdvanced

	// RedisMetricGroupPubSub enables Redis Pub/Sub metrics.
	RedisMetricGroupPubSub = redisotelnative.MetricGroupFlagPubSub

	// RedisMetricGroupStream enables Redis Stream metrics.
	RedisMetricGroupStream = redisotelnative.MetricGroupFlagStream

	// RedisMetricGroupDefault enables production-safe default Redis client metrics.
	RedisMetricGroupDefault = RedisMetricGroupCommand |
		RedisMetricGroupConnectionBasic |
		RedisMetricGroupResiliency |
		RedisMetricGroupConnectionAdvanced

	// RedisMetricGroupAll enables all Redis client metric groups.
	RedisMetricGroupAll = redisotelnative.MetricGroupAll
)

const (
	// RedisHistogramAggregationExplicitBucket uses explicit bucket histograms.
	RedisHistogramAggregationExplicitBucket = redisotelnative.HistogramAggregationExplicitBucket

	// RedisHistogramAggregationBase2Exponential uses base-2 exponential bucket histograms.
	RedisHistogramAggregationBase2Exponential = redisotelnative.HistogramAggregationBase2Exponential
)

// MetricsOption configures OpenTelemetry metrics instrumentation.
type MetricsOption interface {
	apply(cfg *metricsConfig)
}

type metricsOptionFunc func(cfg *metricsConfig)

func (f metricsOptionFunc) apply(cfg *metricsConfig) {
	f(cfg)
}

type metricsConfig struct {
	meterProvider           metric.MeterProvider
	metricGroups            RedisMetricGroupFlags
	includeCommands         []string
	excludeCommands         []string
	hidePubSubChannelNames  bool
	hideStreamNames         bool
	histogramAggregation    RedisHistogramAggregation
	histogramAggregationSet bool
	histogramBuckets        []float64
}

// WithMeterProvider configures the OpenTelemetry meter provider used by native
// go-redis and xredis wrapper-level metrics.
func WithMeterProvider(provider metric.MeterProvider) MetricsOption {
	return metricsOptionFunc(func(cfg *metricsConfig) {
		if provider != nil {
			cfg.meterProvider = provider
		}
	})
}

// WithRedisMetricGroups configures enabled native Redis client metric groups.
func WithRedisMetricGroups(groups RedisMetricGroupFlags) MetricsOption {
	return metricsOptionFunc(func(cfg *metricsConfig) {
		cfg.metricGroups = groups
	})
}

// WithRedisMetricIncludeCommands configures Redis command allow-list for native metrics.
func WithRedisMetricIncludeCommands(commands ...string) MetricsOption {
	return metricsOptionFunc(func(cfg *metricsConfig) {
		cfg.includeCommands = append([]string(nil), commands...)
	})
}

// WithRedisMetricExcludeCommands configures Redis command deny-list for native metrics.
func WithRedisMetricExcludeCommands(commands ...string) MetricsOption {
	return metricsOptionFunc(func(cfg *metricsConfig) {
		cfg.excludeCommands = append([]string(nil), commands...)
	})
}

// WithRedisMetricHidePubSubChannelNames controls Pub/Sub channel name attributes.
func WithRedisMetricHidePubSubChannelNames(hide bool) MetricsOption {
	return metricsOptionFunc(func(cfg *metricsConfig) {
		cfg.hidePubSubChannelNames = hide
	})
}

// WithRedisMetricHideStreamNames controls Stream name attributes.
func WithRedisMetricHideStreamNames(hide bool) MetricsOption {
	return metricsOptionFunc(func(cfg *metricsConfig) {
		cfg.hideStreamNames = hide
	})
}

// WithRedisMetricHistogramAggregation configures native Redis metric histogram aggregation.
func WithRedisMetricHistogramAggregation(aggregation RedisHistogramAggregation) MetricsOption {
	return metricsOptionFunc(func(cfg *metricsConfig) {
		cfg.histogramAggregation = aggregation
		cfg.histogramAggregationSet = true
	})
}

// WithRedisMetricHistogramBuckets configures native Redis metric histogram bucket boundaries in seconds.
func WithRedisMetricHistogramBuckets(buckets ...float64) MetricsOption {
	return metricsOptionFunc(func(cfg *metricsConfig) {
		cfg.histogramBuckets = append([]float64(nil), buckets...)
	})
}
