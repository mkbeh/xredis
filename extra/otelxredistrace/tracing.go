package otelxredistrace

import (
	"github.com/mkbeh/xredis"
	"github.com/redis/go-redis/extra/redisotel/v9"
)

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
	cfg := tracingConfig{}

	for _, opt := range opts {
		if opt != nil {
			opt.apply(&cfg)
		}
	}

	return &Tracing{
		options: append([]redisotel.TracingOption(nil), cfg.options...),
	}
}

// Instrument attaches tracing instrumentation to client.
func (t *Tracing) Instrument(client *xredis.Client) error {
	if t == nil {
		return nil
	}

	return redisotel.InstrumentTracing(client.Raw(), t.options...)
}
