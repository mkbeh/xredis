# otelxredis

OpenTelemetry metrics integration for `github.com/mkbeh/xredis` wrapper-level operations.

```go
metrics, err := otelxredis.NewMetrics(
	otelxredis.WithMeterProvider(meterProvider),
	otelxredis.WithClientID("orders-cache"),
	otelxredis.WithLabel("service", "orders"),
)
if err != nil {
	return err
}

client, err := xredis.NewClient(
	redisOptions,
	xredis.WithMetrics(metrics),
)
```

A `Metrics` instance may be shared by multiple clients that use the same configured attributes. The application remains responsible for the OpenTelemetry `MeterProvider` lifecycle.

The configured client ID is exposed as `xredis.client.id`, and labels are attached to wrapper-level `xredis.*` metrics. Prefer stable, low-cardinality values.

Native `go-redis` metrics are configured separately through `redisotel-native`:

```go
redisMetrics := redisotelnative.NewConfig().
	WithEnabled(true).
	WithMeterProvider(meterProvider)

redisObs := redisotelnative.GetObservabilityInstance()
if err := redisObs.Init(redisMetrics); err != nil {
	return err
}
defer redisObs.Shutdown()
```
