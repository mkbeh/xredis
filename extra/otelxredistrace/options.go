package otelxredistrace

import (
	"github.com/redis/go-redis/extra/redisotel/v9"
	rdb "github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// TracingOption configures OpenTelemetry tracing instrumentation.
type TracingOption interface {
	apply(cfg *tracingConfig)
}

type tracingOptionFunc func(cfg *tracingConfig)

func (f tracingOptionFunc) apply(cfg *tracingConfig) {
	f(cfg)
}

type tracingConfig struct {
	options []redisotel.TracingOption
}

func (c *tracingConfig) add(opt redisotel.TracingOption) {
	c.options = append(c.options, opt)
}

// WithTracerProvider configures the OpenTelemetry tracer provider.
func WithTracerProvider(provider trace.TracerProvider) TracingOption {
	return tracingOptionFunc(func(cfg *tracingConfig) {
		if provider != nil {
			cfg.add(redisotel.WithTracerProvider(provider))
		}
	})
}

// WithDBStatement controls whether raw Redis commands are recorded in spans.
func WithDBStatement(on bool) TracingOption {
	return tracingOptionFunc(func(cfg *tracingConfig) {
		cfg.add(redisotel.WithDBStatement(on))
	})
}

// WithDBSystem configures the db.system tracing attribute.
func WithDBSystem(system string) TracingOption {
	return tracingOptionFunc(func(cfg *tracingConfig) {
		if system != "" {
			cfg.add(redisotel.WithDBSystem(system))
		}
	})
}

// WithAttributes configures additional tracing attributes.
func WithAttributes(attrs ...attribute.KeyValue) TracingOption {
	return tracingOptionFunc(func(cfg *tracingConfig) {
		if len(attrs) > 0 {
			cfg.add(redisotel.WithAttributes(attrs...))
		}
	})
}

// WithCommandFilter configures command filtering for Redis tracing.
func WithCommandFilter(filter func(cmd rdb.Cmder) bool) TracingOption {
	return tracingOptionFunc(func(cfg *tracingConfig) {
		if filter != nil {
			cfg.add(redisotel.WithCommandFilter(filter))
		}
	})
}

// WithCommandsFilter configures pipeline command filtering for Redis tracing.
func WithCommandsFilter(filter func(cmds []rdb.Cmder) bool) TracingOption {
	return tracingOptionFunc(func(cfg *tracingConfig) {
		if filter != nil {
			cfg.add(redisotel.WithCommandsFilter(filter))
		}
	})
}

// WithDialFilter enables or disables filtering of dial commands in tracing.
func WithDialFilter(on bool) TracingOption {
	return tracingOptionFunc(func(cfg *tracingConfig) {
		cfg.add(redisotel.WithDialFilter(on))
	})
}

// WithCallerEnabled controls whether tracing records caller file and line.
func WithCallerEnabled(on bool) TracingOption {
	return tracingOptionFunc(func(cfg *tracingConfig) {
		cfg.add(redisotel.WithCallerEnabled(on))
	})
}
