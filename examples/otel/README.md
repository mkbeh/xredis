# OpenTelemetry Tracing Example

This example shows how to export `xredis` traces and spans to an OTLP-compatible tracing backend.

**This example demonstrates:**

* Configuring an OpenTelemetry `TracerProvider`
* Exporting traces through OTLP HTTP
* Instrumenting HTTP handlers with `otelhttp`
* Passing the tracer provider to `xredis`
* Creating application spans around Redis operations
* Viewing Redis command spans as children of HTTP and application spans
* Recording a failed Redis command as an error span

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
Redis:       localhost:6379
RedisInsight: http://localhost:5540
Jaeger UI:   http://localhost:16686
OTLP HTTP:   http://localhost:4318/v1/traces
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

The HTTP server starts on:

```text
localhost:8080
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

## View traces

Open Jaeger:

```text
http://localhost:16686
```

Select the service:

```text
xredis-otel-example
```

The API responses include `trace_id`, which can be used to locate a specific trace.

The example enables Redis command statements with `WithTracingDBStatement(true)` so the generated spans are easier to
inspect. Avoid recording command statements when Redis values may contain sensitive data.

## Stop services

From the repository root:

```shell
docker compose -f examples/docker-compose.yml --profile otel down --remove-orphans -v
```

Or from this example directory:

```shell
docker compose -f ../docker-compose.yml --profile otel down --remove-orphans -v
```