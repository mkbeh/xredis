package xredis

import (
	redisotelnative "github.com/redis/go-redis/extra/redisotel-native/v9"
	"go.opentelemetry.io/otel"
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

// ObservabilityOption configures Redis client metrics instrumentation.
type ObservabilityOption interface {
	apply(cfg *observabilityConfig)
}

type observabilityOptionFunc func(cfg *observabilityConfig)

func (f observabilityOptionFunc) apply(cfg *observabilityConfig) {
	f(cfg)
}

type observabilityConfig struct {
	enabled                 bool
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

// InitObservability initializes redisotel-native metrics globally.
//
// Call it once during application startup before creating Redis clients.
// The returned function should be called during application shutdown.
func InitObservability(opts ...ObservabilityOption) (func() error, error) {
	cfg := defaultObservabilityConfig()
	for _, opt := range opts {
		if opt != nil {
			opt.apply(cfg)
		}
	}

	if !cfg.enabled {
		return noopObservabilityShutdown, nil
	}

	provider := cfg.meterProvider
	if provider == nil {
		provider = otel.GetMeterProvider()
	}

	wrapperMetrics, err := newMetrics(provider)
	if err != nil {
		return nil, err
	}

	shutdownRedis, err := initRedisMetrics(provider, cfg)
	if err != nil {
		return nil, err
	}

	setMetrics(wrapperMetrics)

	return func() error {
		clearMetrics(wrapperMetrics)

		return shutdownRedis()
	}, nil
}

// WithMetricsEnabled enables or disables all Redis metrics.
//
// This includes native go-redis metrics and xredis wrapper-level metrics.
func WithMetricsEnabled(enabled bool) ObservabilityOption {
	return observabilityOptionFunc(func(cfg *observabilityConfig) {
		cfg.enabled = enabled
	})
}

// WithMeterProvider configures the OpenTelemetry meter provider used by native
// go-redis and wrapper-level metrics.
func WithMeterProvider(provider metric.MeterProvider) ObservabilityOption {
	return observabilityOptionFunc(func(cfg *observabilityConfig) {
		if provider != nil {
			cfg.meterProvider = provider
		}
	})
}

// WithRedisMetricGroups configures enabled Redis client metric groups.
func WithRedisMetricGroups(groups RedisMetricGroupFlags) ObservabilityOption {
	return observabilityOptionFunc(func(cfg *observabilityConfig) {
		cfg.metricGroups = groups
	})
}

// WithRedisMetricIncludeCommands configures Redis command allow-list for metrics.
func WithRedisMetricIncludeCommands(commands ...string) ObservabilityOption {
	return observabilityOptionFunc(func(cfg *observabilityConfig) {
		cfg.includeCommands = append([]string(nil), commands...)
	})
}

// WithRedisMetricExcludeCommands configures Redis command deny-list for metrics.
func WithRedisMetricExcludeCommands(commands ...string) ObservabilityOption {
	return observabilityOptionFunc(func(cfg *observabilityConfig) {
		cfg.excludeCommands = append([]string(nil), commands...)
	})
}

// WithRedisMetricHidePubSubChannelNames controls Pub/Sub channel name attributes.
func WithRedisMetricHidePubSubChannelNames(hide bool) ObservabilityOption {
	return observabilityOptionFunc(func(cfg *observabilityConfig) {
		cfg.hidePubSubChannelNames = hide
	})
}

// WithRedisMetricHideStreamNames controls Stream name attributes.
func WithRedisMetricHideStreamNames(hide bool) ObservabilityOption {
	return observabilityOptionFunc(func(cfg *observabilityConfig) {
		cfg.hideStreamNames = hide
	})
}

// WithRedisMetricHistogramAggregation configures Redis metric histogram aggregation.
func WithRedisMetricHistogramAggregation(aggregation RedisHistogramAggregation) ObservabilityOption {
	return observabilityOptionFunc(func(cfg *observabilityConfig) {
		cfg.histogramAggregation = aggregation
		cfg.histogramAggregationSet = true
	})
}

// WithRedisMetricHistogramBuckets configures Redis metric histogram bucket boundaries in seconds.
func WithRedisMetricHistogramBuckets(buckets ...float64) ObservabilityOption {
	return observabilityOptionFunc(func(cfg *observabilityConfig) {
		cfg.histogramBuckets = append([]float64(nil), buckets...)
	})
}

func initRedisMetrics(
	provider metric.MeterProvider,
	cfg *observabilityConfig,
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

func defaultObservabilityConfig() *observabilityConfig {
	return &observabilityConfig{
		enabled:                true,
		metricGroups:           RedisMetricGroupDefault,
		hidePubSubChannelNames: true,
		hideStreamNames:        true,
	}
}

func noopObservabilityShutdown() error {
	return nil
}
