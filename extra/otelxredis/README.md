# otelxredis

**`otelxredis`** brings seamless OpenTelemetry metrics integration to `github.com/mkbeh/xredis` wrapper-level
operations, automatically instrumenting its built-in cache, distributed locks, and rate limiter.

## Metrics

The `Metrics` type implements the `xredis.Metrics` interface and records measurements through an OpenTelemetry
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

### Metric attributes

The metrics use the following OpenTelemetry attributes:

| Attribute                       | Values                                           | Description                                    |
|:--------------------------------|:-------------------------------------------------|:-----------------------------------------------|
| `xredis.client.id`              | User-defined                                     | Optional client identity.                      |
| `xredis.cache.operation`        | `get`, `get_or_load`                             | Cache operation being performed.               |
| `xredis.cache.result`           | `hit`, `miss`, `negative_hit`, `error`           | Result of the cache lookup.                    |
| `xredis.cache.loader.outcome`   | `success`, `not_found`, `error`                  | Outcome of the cache loader execution.         |
| `xredis.lock.type`              | `lease`, `fenced`                                | Type of distributed lock.                      |
| `xredis.lock.operation`         | `acquire`, `extend`, `unlock`                    | Lock operation being performed.                |
| `xredis.lock.outcome`           | `success`, `contended`, `not_owned`, `error`     | Result of the lock operation.                  |
| `xredis.rate_limiter.algorithm` | `fixed_window`, `sliding_window`, `token_bucket` | Rate-limiting algorithm used for the decision. |
| `xredis.rate_limiter.outcome`   | `allowed`, `rejected`, `error`                   | Result of the rate-limit decision.             |

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

For a complete OpenTelemetry setup with metrics and tracing, see [examples/otel](../../examples/otel).