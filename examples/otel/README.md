## Example: OpenTelemetry

This example demonstrates how to instrument `xredis` with OpenTelemetry metrics and distributed tracing.

### Key Concepts

* **Shared Resource** — Use a common OpenTelemetry `Resource` for metrics and traces.
* **Native Redis Metrics** — Collect `go-redis` client metrics with `redisotel-native` and expose them through Prometheus.
* **xredis Metrics** — Collect wrapper-level cache metrics through `otelxredis` and `WithMetrics`.
* **Distributed Tracing** — Export traces through OTLP HTTP and instrument Redis commands with `redisotel`.
* **HTTP Instrumentation** — Instrument HTTP handlers with `otelhttp` while keeping HTTP metrics disabled.
* **Error Visibility** — Record failed Redis commands and application spans as errors.

## Configuration

```text
REDIS_ADDR=localhost:6379
HTTP_ADDR=localhost:8080
OTEL_SERVICE_NAME=xredis-otel-example
OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=http://localhost:4318/v1/traces
```

The tracing backend is not tied to this example. Any OTLP-compatible backend can be used.

## Local setup

Start standalone Redis, RedisInsight, and Jaeger from the repository root:

```shell
docker compose -f examples/docker-compose.yml --profile otel up -d
```

Or from this example directory:

```shell
docker compose -f ../docker-compose.yml --profile otel up -d
```

Services are available at:

```text
Redis:        localhost:6379
RedisInsight: http://localhost:5540
Jaeger UI:    http://localhost:16686
OTLP HTTP:    http://localhost:4318/v1/traces
```

## Run

From this directory:

```shell
go run .
```

Or from the repository root:

```shell
go run ./examples/otel
```

The HTTP server listens on:

```text
http://localhost:8080
```

Prometheus metrics are exposed at:

```text
http://localhost:8080/metrics
```

## Store a value

```shell
curl -X PUT 'localhost:8080/values/message' \
  -H 'Content-Type: application/json' \
  -d '{
    "value": "hello from xredis"
  }'
```

Expected response:

```json
{
  "key": "xredis:otel:message",
  "trace_id": "...",
  "value": "hello from xredis"
}
```

The trace contains:

```text
PUT /values/{key}
└── store Redis value
    └── Redis SET command span
```

## Read a value

```shell
curl 'localhost:8080/values/message'
```

The trace contains:

```text
GET /values/{key}
└── load Redis value
    └── Redis GET command span
```

The request also records the `xredis` cache lookup metric. Repeating the request after storing a value produces a cache
hit, while reading a missing key produces a cache miss.

## Delete a value

```shell
curl -X DELETE 'localhost:8080/values/message'
```

The trace contains the HTTP span, application span, and Redis `DEL` command span.

## Generate an error span

This endpoint stores a non-integer value and then intentionally executes `INCR` against it:

```shell
curl -X POST 'localhost:8080/errors/counter'
```

Expected result:

```text
HTTP 500
```

In Jaeger, both the application span and the failed Redis command span should be marked as errors.

## Metrics

Open the Prometheus endpoint:

```shell
curl http://localhost:8080/metrics
```

The endpoint exposes both native `go-redis` metrics and `xredis` wrapper-level metrics through the same OpenTelemetry
`MeterProvider`.

Native `go-redis` metrics use the defaults provided by `redisotel-native`. For example, Redis command operations are recorded through `db.client.operation.duration`.

The example also exercises the typed cache so wrapper-level metrics are visible. Depending on the requests sent to the
example, these include:

| Metric | Type | Description |
|---|---|---|
| `xredis_cache_requests_total` | Counter | Cache lookups by operation and result. |
| `xredis_cache_loader_duration_seconds` | Histogram | Cache loader execution duration. |
| `xredis_cache_singleflight_shared_total` | Counter | Requests that received a shared singleflight result. |

Only cache request metrics are exercised by the default value endpoints; loader and singleflight metrics appear when
the corresponding `Cache[T]` workflows are used.

The client ID is exposed as `xredis.client.id`. The `xredis.example` attribute is configured independently for wrapper-level metrics and tracing through `otelxredis.WithLabel` and `redisotel.WithAttributes`. Native `go-redis` metrics are configured separately through `redisotel-native`.

## Traces

Open Jaeger:

```text
http://localhost:16686
```

Select the service:

```text
xredis-otel-example
```

The API responses include `trace_id`, which can be used to locate a specific trace.

The example enables Redis command statements with `redisotel.WithDBStatement(true)` so generated Redis command spans are easier
to inspect. Avoid recording command statements when Redis values may contain sensitive data.

## Stop services

From the repository root:

```shell
docker compose -f examples/docker-compose.yml --profile otel down --remove-orphans -v
```

Or from this example directory:

```shell
docker compose -f ../docker-compose.yml --profile otel down --remove-orphans -v
```
