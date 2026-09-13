# otelxredistrace

OpenTelemetry tracing integration for `github.com/mkbeh/xredis`.

```go
tracing := otelxredistrace.New(
	otelxredistrace.WithTracerProvider(tracerProvider),
)

client, err := xredis.NewClient(
	xredis.WithTracing(tracing),
)
```

A `Tracing` instance is immutable after construction and may be shared by
multiple clients.
