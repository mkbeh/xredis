# otelxredis

OpenTelemetry metrics integration for `github.com/mkbeh/xredis`.

The module combines native `go-redis` metrics from `redisotel-native` with
wrapper-level `xredis` metrics for caches, locks, and rate limiters.

```go
metrics, err := otelxredis.InitMetrics(
	otelxredis.WithMeterProvider(meterProvider),
)
if err != nil {
	return err
}
defer metrics.Shutdown()

client, err := xredis.NewClient(
	xredis.WithMetrics(metrics),
)
```

A `Metrics` instance may be shared by multiple clients. The application remains
responsible for shutting down its OpenTelemetry `MeterProvider`.
