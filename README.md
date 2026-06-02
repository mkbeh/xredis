# xredis

![Go Version](https://img.shields.io/badge/go-1.26+-blue)
![License](https://img.shields.io/badge/license-MIT-green)

A Go Redis client wrapper built on [go-redis](https://github.com/redis/go-redis) with built-in OpenTelemetry tracing,
Prometheus metrics, and support for both standalone and cluster modes.

## Features

- Standalone and cluster client support
- OpenTelemetry tracing and metrics via `redisotel`
- Prometheus metrics via custom collector
- TLS support
- Rate limiting via `rdb.Limiter`
- Configurable marshaller (defaults to `json.Marshal`)
- Hash operations with struct mapping
- Pipeline-based bulk delete

## Installation

```bash
go get github.com/mkbeh/xredis
```

## Quick start

```go
package main

import (
	"context"
	"fmt"

	redis "github.com/mkbeh/xredis"
)

func main() {
	client, err := redis.NewClient(
		redis.WithConfig(&redis.Config{
			Addrs: "localhost:6379",
		}),
		redis.WithClientID("my-service"),
	)
	if err != nil {
		panic(err)
	}
	defer client.Close()

	ctx := context.Background()

	if err := client.Set(ctx, "greeting", "hello", 0); err != nil {
		panic(err)
	}

	var value string
	if err := client.Get(ctx, "greeting", &value); err != nil {
		panic(err)
	}

	fmt.Println(value) // hello
}
```

More examples: [examples/sample](https://github.com/mkbeh/xredis/tree/main/examples/sample)

## Cluster mode

```go
client, err := redis.NewClusterClient(
    redis.WithConfig(&redis.Config{
        Addrs: "node1:6379,node2:6379,node3:6379",
    }),
)
```

## Hash operations

```go
// Set individual hash field
client.HSet(ctx, "user:1", "name", "Alice", time.Hour)

// Set all fields from a struct
type User struct {
    Name  string `redis:"name"`
    Email string `redis:"email"`
}
client.HSetObject(ctx, "user:1", User{Name: "Alice", Email: "alice@example.com"}, time.Hour)

// Get all fields into a struct
var u User
client.HGetAll(ctx, "user:1", &u)
```

## Observability

```go
client, err := redis.NewClient(
    redis.WithConfig(cfg),
    redis.WithTraceProvider(tp),
    redis.WithMeterProvider(mp),
    redis.WithMetricsNamespace("myapp"),
    redis.WithDBStatement(false), // hide raw commands from traces
)
```

Prometheus metrics are registered automatically on client creation.

## TLS

```go
client, err := redis.NewClient(
    redis.WithConfig(cfg),
    redis.WithTLS(&tls.Config{
    MinVersion: tls.VersionTLS12,
}),
)
```

## Options reference

| Option                     | Description                                                       |
|----------------------------|-------------------------------------------------------------------|
| `WithConfig(cfg)`          | Connection and pool configuration                                 |
| `WithClientID(id)`         | Human-readable client name prefix (UUID appended automatically)   |
| `WithLogger(l)`            | Custom `slog.Logger`                                              |
| `WithMarshaller(fn)`       | Custom marshal function for `SetStruct` (default: `json.Marshal`) |
| `WithTLS(cfg)`             | TLS configuration                                                 |
| `WithLimiter(l)`           | Rate limiter (standalone mode only)                               |
| `WithTraceProvider(tp)`    | OpenTelemetry tracer provider                                     |
| `WithMeterProvider(mp)`    | OpenTelemetry meter provider                                      |
| `WithMetricsNamespace(ns)` | Prometheus metrics namespace                                      |
| `WithDBStatement(on)`      | Include raw commands in traces                                    |
| `WithDBSystem(s)`          | Override `db.system` attribute in traces and metrics              |
| `WithAttributes(attrs...)` | Additional OpenTelemetry attributes for traces and metrics        |

## Configuration

All fields can be set programmatically via `Config` or through environment variables.

| Env variable                    | Default          | Description                                               |
|---------------------------------|------------------|-----------------------------------------------------------|
| `REDIS_ADDRS`                   | `127.0.0.1:6379` | Comma-separated list of `host:port` addresses             |
| `REDIS_NETWORK`                 | `tcp`            | Network type: `tcp` or `unix`                             |
| `REDIS_PROTOCOL`                | `3`              | RESP protocol version: `2` or `3`                         |
| `REDIS_USERNAME`                | —                | ACL username (Redis 6+)                                   |
| `REDIS_PASSWORD`                | —                | Password                                                  |
| `REDIS_DB`                      | `0`              | Database index (standalone only)                          |
| `REDIS_MAX_RETRIES`             | `3`              | Max retries; `-1` disables                                |
| `REDIS_MIN_RETRY_BACKOFF`       | `8ms`            | Min backoff between retries; `-1` disables                |
| `REDIS_MAX_RETRY_BACKOFF`       | `512ms`          | Max backoff between retries; `-1` disables                |
| `REDIS_MAX_REDIRECTS`           | `len(nodes)+1`   | Max MOVED/ASK redirects (cluster only)                    |
| `REDIS_READONLY`                | `true`           | Route read commands to replicas (cluster only)            |
| `REDIS_ROUTE_BY_LATENCY`        | `false`          | Route reads to the nearest node (cluster only)            |
| `REDIS_ROUTE_RANDOMLY`          | `false`          | Route reads to a random node (cluster only)               |
| `REDIS_DIAL_TIMEOUT`            | `5s`             | Timeout for new connections                               |
| `REDIS_READ_TIMEOUT`            | `3s`             | Socket read timeout; `-1` blocks; `-2` disables deadline  |
| `REDIS_WRITE_TIMEOUT`           | `3s`             | Socket write timeout; `-1` blocks; `-2` disables deadline |
| `REDIS_CONTEXT_TIMEOUT_ENABLED` | `false`          | Respect context deadlines for commands                    |
| `REDIS_POOL_SIZE`               | `10×GOMAXPROCS`  | Connection pool size per node                             |
| `REDIS_POOL_FIFO`               | `false`          | `true` = FIFO pool; `false` = LIFO                        |
| `REDIS_POOL_TIMEOUT`            | `ReadTimeout+1s` | Wait time for a free connection                           |
| `REDIS_MIN_IDLE_CONNS`          | `0`              | Minimum idle connections                                  |
| `REDIS_MAX_IDLE_CONNS`          | `0`              | Maximum idle connections                                  |
| `REDIS_MAX_ACTIVE_CONNS`        | `0` (unlimited)  | Maximum active connections per node                       |
| `REDIS_CONN_MAX_IDLE_TIME`      | `30m`            | Max idle time per connection; `-1` disables               |
| `REDIS_CONN_MAX_LIFETIME`       | unlimited        | Max lifetime per connection                               |
| `REDIS_DISABLE_INDENTITY`       | `false`          | Disable `CLIENT SETNAME` on connect                       |
| `REDIS_UNSTABLE_RESP3`          | `false`          | Enable unstable RESP3 mode for Redis Search               |

## Error handling

```go
import "errors"

val, ok, err := client.String(ctx, "key")
if err != nil {
    // real error
}
if !ok {
    // key does not exist
}

err = client.HGetAll(ctx, "key", &dst)
if errors.Is(err, redis.ErrKeyNotFound) {
    // key does not exist
}
```

Exported errors:

| Error                 | Description                               |
|-----------------------|-------------------------------------------|
| `ErrKeyNotFound`      | Key or field does not exist               |
| `ErrInvalidFieldType` | Unsupported field type for hash operation |

## License

[MIT](LICENSE)