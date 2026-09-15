# otelxredis

**`otelxredis`** brings seamless OpenTelemetry metrics integration to `github.com/mkbeh/xredis` wrapper-level
operations, automatically instrumenting its built-in cache, distributed locks, and rate limiter.

## Metrics

The `Metrics` type implements the xredis metrics interfaces and records measurements through an OpenTelemetry
`MeterProvider`, automatically attaching configured static attributes to all emitted metrics.

| Metric                             | Type      | Description / Attributes                                                                                  |
|------------------------------------|-----------|-----------------------------------------------------------------------------------------------------------|
| Cache                              |           |                                                                                                           |
| `xredis.cache.requests`            | Counter   | Cache requests (`xredis.cache.operation`, `xredis.cache.result`).                                         |
| `xredis.cache.loader.duration`     | Histogram | Cache loader execution duration in seconds (`xredis.cache.loader.outcome`).                               |
| `xredis.cache.singleflight.shared` | Counter   | Cache requests that shared a singleflight result.                                                         |
| Distributed Locks                  |           |                                                                                                           |
| `xredis.lock.operations`           | Counter   | Lock operations (`xredis.lock.type`, `xredis.lock.operation`, `xredis.lock.outcome`).                     |
| Rate Limiter                       |           |                                                                                                           |
| `xredis.rate_limiter.decisions`    | Counter   | Rate limit decisions (`xredis.rate_limiter.algorithm`, `xredis.rate_limiter.outcome`).                    |
| `xredis.rate_limiter.duration`     | Histogram | Rate limit decision duration in seconds (`xredis.rate_limiter.algorithm`, `xredis.rate_limiter.outcome`). |

### Getting started

To start using `otelxredis` metrics, you will need to:

1. Set up a meter provider.
2. Create an `otelxredis` metrics instance.
3. Register the metrics with an `xredis` client.

Here's an example of how you might do this:

<!-- @formatter:off -->

```go
// Initialize meter provider.
meterProvider, err := initMeterProvider()

// Create a new otelxredis metrics.
metrics, err := otelxredis.NewMetrics(
    otelxredis.WithMeterProvider(meterProvider),
)

// Create new xredis client with metrics.
client, err := xredis.NewClient(
    redisOptions,

    // Register metrics.
    xredis.WithMetrics(metrics),
)
```

<!-- @formatter:on -->
