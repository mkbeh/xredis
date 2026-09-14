# otelxredistrace

OpenTelemetry tracing integration for `github.com/mkbeh/xredis`.

```go
tracing := otelxredistrace.New(
	otelxredistrace.WithTracerProvider(tracerProvider),
	otelxredistrace.WithClientID("orders-cache"),
	otelxredistrace.WithLabel("service", "orders"),
)

client, err := xredis.NewClient(
	xredis.WithClientName("orders-cache"),
	xredis.WithTracing(tracing),
)
```

A `Tracing` instance is immutable after construction and may be shared by
multiple clients.

The configured client ID is exposed as `xredis.client.id`. Labels and attributes
are attached to Redis spans. The application may use the same value for the
telemetry client ID and Redis client name while keeping the two concepts
separate.
