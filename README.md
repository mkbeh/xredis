<div align="center">

# xredis

**Lightweight Redis wrapper for Go, built on top of [go-redis](https://github.com/redis/go-redis).**

[![Go Reference](https://pkg.go.dev/badge/github.com/mkbeh/xredis.svg)](https://pkg.go.dev/github.com/mkbeh/xredis)
![Go Version](https://img.shields.io/badge/go-1.26%2B-blue)
[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

</div>

`xredis` wraps the excellent [`go-redis`](https://github.com/redis/go-redis) client with a compact API for common Redis
workflows: standalone and cluster clients, typed value helpers, hash mapping, rate limiting, TLS configuration,
pipeline-based bulk deletion, and Redis observability with OpenTelemetry and Prometheus.

## Features

* **Clients**: Standalone and cluster Redis client support.
* **Values**: Typed helpers for reading and writing Redis values.
* **Hashes**: Hash operations with struct mapping.
* **Pipelines**: Pipeline-based bulk delete helpers.
* **Limiting**: Rate limiting via `redis.Limiter` for standalone clients.
* **Observability**: OpenTelemetry tracing and metrics through `redisotel` and Prometheus metrics.
* **Security**: TLS configuration support.
* **Configuration**: Configure via Go structs or environment variables.

## Installation

```bash
go get github.com/mkbeh/xredis
```

## Quick start

The example below creates a standalone Redis client and stores a simple value.

```go
package main

import (
	"context"
	"fmt"
	"log"

	redis "github.com/mkbeh/xredis"
)

func main() {
	ctx := context.Background()

	client, err := redis.NewClient(
		redis.WithConfig(&redis.Config{
			Addrs: "localhost:6379",
		}),
		redis.WithClientID("my-service"),
	)
	if err != nil {
		log.Fatal("failed to init Redis client:", err)
	}
	defer client.Close()

	if err := client.Set(ctx, "greeting", "hello", 0); err != nil {
		log.Fatal("set failed:", err)
	}

	var value string
	if err := client.Get(ctx, "greeting", &value); err != nil {
		log.Fatal("get failed:", err)
	}

	fmt.Println(value)
}
```

More examples: [examples/sample](https://github.com/mkbeh/xredis/tree/main/examples/sample)

## Typed Values

`xredis` provides typed helpers that return a clean `ok` flag when a key is missing, removing the need to check for raw
`redis.Nil` errors manually.

<!-- @formatter:off -->
```go
value, ok, err := client.String(ctx, "greeting")
if err != nil {
	return fmt.Errorf("failed to fetch greeting: %w", err)
}
if !ok {
	// key does not exist
	return nil
}

fmt.Println(value)
```
<!-- @formatter:on -->

Available typed helpers include: `String`, `Bool`, `Bytes`, `Float64`, `Int`, `Int64`, and `Uint64`.

## Struct Values

Use `SetStruct` to store structured data using a configurable marshaller. By default, it uses standard `json.Marshal`,
but you can easily override it.

<!-- @formatter:off -->
```go
type Session struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

session := Session{
	UserID: "user-1",
	Role:   "admin",
}

if err := client.SetStruct(ctx, "session:1", session, time.Hour); err != nil {
	log.Fatalf("failed to store session struct: %v", err)
}
```
<!-- @formatter:on -->


To inject a custom encoder (e.g., Protobuf, MessagePack, or an optimized JSON library), use `WithMarshaller`:

<!-- @formatter:off -->
```go
client, err := redis.NewClient(
	redis.WithConfig(cfg),
	redis.WithMarshaller(customMarshal),
)
if err != nil {
	log.Fatalf("failed to initialize xredis client: %v", err)
}
defer client.Close()
```
<!-- @formatter:on -->

## Hash Operations

`xredis` simplifies working with Redis hashes by providing seamless struct-to-hash and hash-to-struct mapping utilities.

<!-- @formatter:off -->
```go
type User struct {
	Name  string `redis:"name"`
	Email string `redis:"email"`
}

user := User{
	Name:  "Alice",
	Email: "alice@example.com",
}

// 1. Set a single hash field
if err := client.HSet(ctx, "user:1", "name", user.Name, time.Hour); err != nil {
	log.Fatalf("failed to set hash field: %v", err)
}

// 2. Automatically map entire struct fields into a Redis Hash
if err := client.HSetObject(ctx, "user:1", user, time.Hour); err != nil {
	log.Fatalf("failed to set hash object from struct: %v", err)
}

// 3. Scan Redis Hash fields back into a Go struct directly
var dst User
if err := client.HGetAll(ctx, "user:1", &dst); err != nil {
	log.Fatalf("failed to scan hash fields into struct: %v", err)
}
```
<!-- @formatter:on -->

## Bulk Delete

Use `MassDelete` to safely clear multiple keys within a single network round-trip using an atomic Redis pipeline.

<!-- @formatter:off -->
```go
keys := []string{
	"cache:user:1",
	"cache:user:2",
	"cache:user:3",
}

if err := client.MassDelete(ctx, keys); err != nil {
	log.Fatalf("pipeline mass delete failed: %v", err)
}
```
<!-- @formatter:on -->

## Observability

`xredis` instruments Redis commands through `redisotel` and exposes Prometheus metrics out of the box.

<!-- @formatter:off -->
```go
client, err := redis.NewClient(
	redis.WithConfig(cfg),
	redis.WithClientID("my-service"),
	redis.WithTraceProvider(tracerProvider),
	redis.WithMeterProvider(meterProvider),
	redis.WithMetricsNamespace("myapp"),
	redis.WithDBStatement(false), // hide raw Redis commands from traces
)
if err != nil {
	log.Fatalf("failed to initialize observed redis client: %v", err)
}
defer client.Close()
```
<!-- @formatter:on -->

Prometheus metrics are registered automatically on client creation.

## TLS Support

Pass a custom TLS configuration via `WithTLS` to secure your connection.

<!-- @formatter:off -->
```go
client, err := redis.NewClient(
	redis.WithConfig(cfg),
	redis.WithTLS(&tls.Config{
		MinVersion: tls.VersionTLS12,
	}),
)
if err != nil {
	log.Fatalf("failed to initialize secure redis client: %v", err)
}
defer client.Close()
```
<!-- @formatter:on -->

## Rate Limiting

Standalone clients can inject a custom `go-redis` rate limiter to control request throughput.

<!-- @formatter:off -->
```go
client, err := redis.NewClient(
	redis.WithConfig(cfg),
	redis.WithLimiter(limiter),
)
if err != nil {
	log.Fatalf("failed to initialize rate-limited redis client: %v", err)
}
defer client.Close()
```
<!-- @formatter:on -->

## Error Handling

`xredis` provides explicit flags for simple lookups and exports normalized errors for complex operations, eliminating
the need to import underlying driver errors manually.

### Missing Keys (Typed Helpers)

For basic data types, helpers return an explicit `ok` flag to handle missing keys cleanly without triggering an error.

<!-- @formatter:off -->
```go
value, ok, err := client.String(ctx, "greeting")
if err != nil {
	return fmt.Errorf("failed to fetch greeting: %w", err)
}
if !ok {
	// key does not exist
	return nil
}
```
<!-- @formatter:on -->

### Missing Keys (Hash & Object Helpers)

Hash and object operations return exported, predictable errors when a key or field is missing:

<!-- @formatter:off -->
```go
err := client.HGetAll(ctx, "user:1", &dst)
if errors.Is(err, redis.ErrKeyNotFound) {
	// key does not exist
	return nil
}
if err != nil {
	return fmt.Errorf("hash operation failed: %w", err)
}
```
<!-- @formatter:on -->

### Exported Errors Reference

| Error | Description |
| :--- | :--- |
| `ErrKeyNotFound` | The requested key or hash field does not exist. |
| `ErrInvalidFieldType` | The struct field type is unsupported for hash mapping operations. |

## Configuration

The `Config` struct can be initialized directly in Go. It also includes `envconfig` tags, allowing you to seamlessly
populate it from environment variables using your preferred configuration library.

### Config Struct

<!-- @formatter:off -->
```go
cfg := &redis.Config{
    Addrs:    "localhost:6379",
    Network:  "tcp",
    Protocol: 3,

    Username: "user",
    Password: "password",
    DB:       0, // standalone mode only

    MaxRetries:      3,
    MinRetryBackoff: 8 * time.Millisecond,
    MaxRetryBackoff: 512 * time.Millisecond,

    DialTimeout:  5 * time.Second,
    ReadTimeout:  3 * time.Second,
    WriteTimeout: 3 * time.Second,

    PoolSize:        10 * runtime.GOMAXPROCS(0),
    MinIdleConns:    0,
    MaxIdleConns:    0,
    MaxActiveConns:  0,
    ConnMaxIdleTime: 30 * time.Minute,

    DisableIdentity: false,
}
```
<!-- @formatter:on -->

### Environment Variables

| Env Variable | Default | Description |
| :--- | :--- | :--- |
| `REDIS_ADDRS` | `127.0.0.1:6379` | Comma-separated list of `host:port` addresses. |
| `REDIS_NETWORK` | `tcp` | Network type: `tcp` or `unix`. |
| `REDIS_PROTOCOL` | `3` | RESP protocol version: `2` or `3`. |
| `REDIS_USERNAME` | — | ACL username for Redis 6+. |
| `REDIS_PASSWORD` | — | Password. |
| `REDIS_DB` | `0` | Database index, standalone mode only. |
| `REDIS_MAX_RETRIES` | `3` | Maximum retries; `-1` disables retries. |
| `REDIS_MIN_RETRY_BACKOFF` | `8ms` | Minimum backoff between retries; `-1` disables backoff. |
| `REDIS_MAX_RETRY_BACKOFF` | `512ms` | Maximum backoff between retries; `-1` disables backoff. |
| `REDIS_MAX_REDIRECTS` | `len(nodes)+1` | Maximum `MOVED` / `ASK` redirects, cluster mode only. |
| `REDIS_READONLY` | `true` | Route read commands to replicas, cluster mode only. |
| `REDIS_ROUTE_BY_LATENCY` | `false` | Route reads to the nearest node, cluster mode only. |
| `REDIS_ROUTE_RANDOMLY` | `false` | Route reads to a random node, cluster mode only. |
| `REDIS_DIAL_TIMEOUT` | `5s` | Timeout for new connections. |
| `REDIS_READ_TIMEOUT` | `3s` | Socket read timeout; `-1` blocks, `-2` disables deadline. |
| `REDIS_WRITE_TIMEOUT` | `3s` | Socket write timeout; `-1` blocks, `-2` disables deadline. |
| `REDIS_CONTEXT_TIMEOUT_ENABLED` | `false` | Respect context deadlines for commands. |
| `REDIS_POOL_SIZE` | `10 * GOMAXPROCS` | Connection pool size per node. |
| `REDIS_POOL_FIFO` | `false` | `true` = FIFO pool, `false` = LIFO pool. |
| `REDIS_POOL_TIMEOUT` | `ReadTimeout+1s` | Wait time for a free connection. |
| `REDIS_MIN_IDLE_CONNS` | `0` | Minimum idle connections. |
| `REDIS_MAX_IDLE_CONNS` | `0` | Maximum idle connections. |
| `REDIS_MAX_ACTIVE_CONNS` | `0` | Maximum active connections per node; `0` means unlimited. |
| `REDIS_CONN_MAX_IDLE_TIME` | `30m` | Maximum idle time per connection; `-1` disables it. |
| `REDIS_CONN_MAX_LIFETIME` | unlimited | Maximum lifetime per connection. |
| `REDIS_DISABLE_IDENTITY` | `false` | Disable `CLIENT SETNAME` on connect. |
| `REDIS_UNSTABLE_RESP3` | `false` | Enable unstable RESP3 mode for Redis Search. |

## License

This project is licensed under the [MIT License](LICENSE).