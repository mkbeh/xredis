package otelxredis

import (
	redisotelnative "github.com/redis/go-redis/extra/redisotel-native/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
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
			opt.apply(cfg)
		}
	}

	provider := cfg.meterProvider
	if provider == nil {
		provider = otel.GetMeterProvider()
	}

	metrics, err := newMetrics(provider)
	if err != nil {
		return nil, err
	}

	shutdownNative, err := initNativeMetrics(provider, cfg)
	if err != nil {
		return nil, err
	}

	metrics.shutdownNative = shutdownNative

	return metrics, nil
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

func defaultMetricsConfig() *metricsConfig {
	return &metricsConfig{
		metricGroups:           RedisMetricGroupDefault,
		hidePubSubChannelNames: true,
		hideStreamNames:        true,
	}
}
