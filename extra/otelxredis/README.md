# otelxredis

OpenTelemetry metrics integration for `github.com/mkbeh/xredis`.

The module combines native `go-redis` metrics from `redisotel-native` with
wrapper-level `xredis` metrics for caches, locks, and rate limiters.

```go
metrics, err := otelxredis.InitMetrics(
	otelxredis.WithMeterProvider(meterProvider),
	otelxredis.WithClientID("orders-cache"),
	otelxredis.WithLabel("service", "orders"),
)
if err != nil {
	return err
}
defer metrics.Shutdown()

client, err := xredis.NewClient(
	xredis.WithClientName("orders-cache"),
	xredis.WithMetrics(metrics),
)
```

A `Metrics` instance may be shared by multiple clients. The application remains
responsible for shutting down its OpenTelemetry `MeterProvider`.

The configured client ID is exposed as `xredis.client.id`, and client labels are
attached to wrapper-level `xredis.*` metrics. Prefer stable, low-cardinality
values when using labels with metrics.

Native `go-redis` metrics keep the attributes provided by `redisotel-native`.
