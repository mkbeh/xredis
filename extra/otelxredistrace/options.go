package otelxredistrace

import (
	"slices"

	"github.com/redis/go-redis/extra/redisotel/v9"
	rdb "github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// TracingOption configures OpenTelemetry tracing instrumentation.
type TracingOption func(*tracingConfig)

type tracingConfig struct {
	clientID   string
	labels     map[string]string
	attributes []attribute.KeyValue
	options    []redisotel.TracingOption
}

func defaultTracingConfig() tracingConfig {
	return tracingConfig{
		labels: make(map[string]string),
	}
}

func (c *tracingConfig) add(opt redisotel.TracingOption) {
	c.options = append(c.options, opt)
}

// WithTracerProvider configures the OpenTelemetry tracer provider.
func WithTracerProvider(provider trace.TracerProvider) TracingOption {
	return func(cfg *tracingConfig) {
		if provider != nil {
			cfg.add(redisotel.WithTracerProvider(provider))
		}
	}
}

// WithClientID configures the client identity attribute for Redis spans.
func WithClientID(id string) TracingOption {
	return func(cfg *tracingConfig) {
		if id != "" {
			cfg.clientID = id
		}
	}
}

// WithLabel adds a string attribute to Redis spans.
func WithLabel(key, value string) TracingOption {
	return func(cfg *tracingConfig) {
		if key != "" {
			cfg.labels[key] = value
		}
	}
}

// WithLabels adds string attributes to Redis spans.
//
// Labels are merged with previously configured labels. When the same key is
// configured more than once, the last value wins.
func WithLabels(labels map[string]string) TracingOption {
	return func(cfg *tracingConfig) {
		for key, value := range labels {
			if key != "" {
				cfg.labels[key] = value
			}
		}
	}
}

// WithDBStatement controls whether raw Redis commands are recorded in spans.
func WithDBStatement(on bool) TracingOption {
	return func(cfg *tracingConfig) {
		cfg.add(redisotel.WithDBStatement(on))
	}
}

// WithDBSystem configures the db.system tracing attribute.
func WithDBSystem(system string) TracingOption {
	return func(cfg *tracingConfig) {
		if system != "" {
			cfg.add(redisotel.WithDBSystem(system))
		}
	}
}

// WithAttributes configures additional tracing attributes.
func WithAttributes(attrs ...attribute.KeyValue) TracingOption {
	return func(cfg *tracingConfig) {
		cfg.attributes = append(cfg.attributes, slices.Clone(attrs)...)
	}
}

// WithCommandFilter configures command filtering for Redis tracing.
func WithCommandFilter(filter func(cmd rdb.Cmder) bool) TracingOption {
	return func(cfg *tracingConfig) {
		if filter != nil {
			cfg.add(redisotel.WithCommandFilter(filter))
		}
	}
}

// WithCommandsFilter configures pipeline command filtering for Redis tracing.
func WithCommandsFilter(filter func(cmds []rdb.Cmder) bool) TracingOption {
	return func(cfg *tracingConfig) {
		if filter != nil {
			cfg.add(redisotel.WithCommandsFilter(filter))
		}
	}
}

// WithDialFilter enables or disables filtering of dial commands in tracing.
func WithDialFilter(on bool) TracingOption {
	return func(cfg *tracingConfig) {
		cfg.add(redisotel.WithDialFilter(on))
	}
}

// WithCallerEnabled controls whether tracing records caller file and line.
func WithCallerEnabled(on bool) TracingOption {
	return func(cfg *tracingConfig) {
		cfg.add(redisotel.WithCallerEnabled(on))
	}
}
