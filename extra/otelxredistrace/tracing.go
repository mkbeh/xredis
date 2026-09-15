package otelxredistrace

import (
	"slices"

	"github.com/mkbeh/xredis"
	"github.com/redis/go-redis/extra/redisotel/v9"
	"go.opentelemetry.io/otel/attribute"
)

const attrClientID attribute.Key = "xredis.client.id"

// Tracing provides OpenTelemetry tracing instrumentation for xredis clients.
//
// A Tracing instance is immutable after construction and may be shared by
// multiple clients.
type Tracing struct {
	options []redisotel.TracingOption
}

var _ xredis.Tracing = (*Tracing)(nil)

// New creates reusable OpenTelemetry tracing instrumentation.
func New(opts ...TracingOption) *Tracing {
	cfg := defaultTracingConfig()

	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	options := slices.Clone(cfg.options)
	if attributes := attributesFromConfig(&cfg); len(attributes) > 0 {
		options = append(options, redisotel.WithAttributes(attributes...))
	}

	return &Tracing{
		options: options,
	}
}

// Instrument attaches tracing instrumentation to client.
func (t *Tracing) Instrument(client *xredis.Client) error {
	if t == nil {
		return nil
	}

	return redisotel.InstrumentTracing(client.Raw(), t.options...)
}

func attributesFromConfig(cfg *tracingConfig) []attribute.KeyValue {
	attrs := make([]attribute.KeyValue, 0, len(cfg.labels)+len(cfg.attributes)+1)

	for key, value := range cfg.labels {
		attrs = append(attrs, attribute.String(key, value))
	}

	attrs = append(attrs, cfg.attributes...)

	if cfg.clientID != "" {
		attrs = append(attrs, attrClientID.String(cfg.clientID))
	}

	set := attribute.NewSet(attrs...)

	return set.ToSlice()
}
