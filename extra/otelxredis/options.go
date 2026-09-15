package otelxredis

import (
	"maps"

	"go.opentelemetry.io/otel/metric"
)

// MetricsOption configures Metrics created by NewMetrics.
type MetricsOption func(*metricsConfig)

type metricsConfig struct {
	meterProvider metric.MeterProvider
	clientID      string
	labels        map[string]string
}

func defaultMetricsConfig() metricsConfig {
	return metricsConfig{
		labels: make(map[string]string),
	}
}

// WithMeterProvider sets the OpenTelemetry MeterProvider used to create xredis
// metrics.
//
// A nil provider is ignored. If no provider is configured, NewMetrics uses
// the global OpenTelemetry MeterProvider.
func WithMeterProvider(provider metric.MeterProvider) MetricsOption {
	return func(cfg *metricsConfig) {
		if provider != nil {
			cfg.meterProvider = provider
		}
	}
}

// WithClientID sets the xredis.client.id attribute on xredis metrics.
//
// An empty ID is ignored.
func WithClientID(id string) MetricsOption {
	return func(cfg *metricsConfig) {
		if id != "" {
			cfg.clientID = id
		}
	}
}

// WithLabel adds a static attribute to xredis metrics.
//
// An empty key is ignored. When the same label key is configured more than
// once, the last value wins.
//
// The xredis. attribute namespace is reserved for instrumentation emitted by
// this package. Prefer stable, low-cardinality values.
func WithLabel(key, value string) MetricsOption {
	return func(cfg *metricsConfig) {
		if key != "" {
			cfg.labels[key] = value
		}
	}
}

// WithLabels adds static attributes to xredis metrics.
//
// The map is copied when the option is created, so later changes to the
// original map do not affect the option. Empty keys are ignored. When the same
// label key is configured more than once, the last value wins.
//
// The xredis. attribute namespace is reserved for instrumentation emitted by
// this package. Prefer stable, low-cardinality values.
func WithLabels(labels map[string]string) MetricsOption {
	labels = maps.Clone(labels)

	return func(cfg *metricsConfig) {
		for key, value := range labels {
			if key != "" {
				cfg.labels[key] = value
			}
		}
	}
}
