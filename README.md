<div align="center">

# Redis toolkit for Go

**Lightweight Redis wrapper for Go, built on top of [go-redis](https://github.com/redis/go-redis).**

[![Go](https://github.com/mkbeh/xredis/actions/workflows/go.yml/badge.svg?branch=main)](https://github.com/mkbeh/xredis/actions/workflows/go.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/mkbeh/xredis.svg)](https://pkg.go.dev/github.com/mkbeh/xredis)
[![codecov](https://codecov.io/gh/mkbeh/xredis/branch/main/graph/badge.svg)](https://codecov.io/gh/mkbeh/xredis)

</div>

`xredis` builds on the [go-redis](https://github.com/redis/go-redis) client with a compact API for common Redis
workflows and application-level reliability patterns. It provides focused helpers for native and structured values,
typed caching, atomic compare operations (CAS/CAD), lease and fenced locks, distributed rate limiting, bulk pipelines,
topology-wide scans, and OpenTelemetry metrics and tracing.

The library preserves the topology support and core behavior of `go-redis` while reducing boilerplate around common
usage patterns.

Runnable examples are available in the [examples](examples) directory.

## Features

* **Multiple topologies** — standalone Redis, Redis Cluster, Sentinel/failover, failover Cluster, and client-side
  sharded Ring deployments.
* **Native and structured values** — standard Redis scalar types use native `go-redis` encoding and scanning, while
  structured values use a configurable `Codec`.
* **Command helpers** — typed value readers, conditional writes, atomic counters, existence checks, deletion helpers,
  and struct-to-hash mapping.
* **Typed cache-aside** — generic `Cache[T]` workflows with TTL jitter, negative caching, configurable not-found
  detection, and singleflight deduplication for concurrent misses.
* **Atomic compare operations** — Lua-backed compare-and-swap (CAS) and compare-and-delete (CAD) operations for exact
  Redis string values and individual hash fields.
* **Versioned structured values** — generic `VersionedStore[T]` workflows with opaque revision tokens, atomic
  initialization through `SetIfAbsent`, optimistic updates, conditional deletion, and configurable expiration.
* **Distributed locks** — token-based lease locks and fenced locks with monotonically increasing fencing tokens.
* **Distributed rate limiting** — atomic fixed window, sliding window, and token bucket algorithms implemented with
  server-side Lua scripts.
* **Bulk operations and pipelines** — helpers for batched key-value writes, structured values, hashes, deletion, and
  unlink operations.
* **Topology-wide scans** — cursor-based iteration across Redis Cluster masters and Redis Ring shards, with type
  filtering and per-key or per-batch handlers.
* **Distributed tracing** — OpenTelemetry command tracing through `redisotel`, with configurable filters, attributes,
  and caller information.
* **Metrics** — native `go-redis` metrics together with wrapper-level OpenTelemetry instrumentation for caches, locks,
  and rate limiters.
* **Production configuration** — TLS and mTLS, ACL authentication, dynamic credential providers, retries, backoff,
  timeouts, custom dialers, and hooks.

## Installation

This repository contains the core `xredis` module. The core package is released from the repository root:

```bash
go get github.com/mkbeh/xredis
```

## Quick start

The following example demonstrates how to initialize the `xredis` client with client options, verify the connection, and
perform basic key-value operations with TTL support.


<!-- @formatter:off -->
```go
ctx := context.Background()

// Initialize the client
client, err := xredis.NewClient(
    xredis.WithClientConfig(&xredis.ClientConfig{
        Addr: "localhost:6379",
        DB:   0,
    }),
    xredis.WithClientID("example-client"),
)
if err != nil {
    return err
}
defer func() {
    if err := client.Close(); err != nil {
        log.Println("unable to close Redis client:", err)
    }
}()

// Verify connection
if err := client.Ping(ctx); err != nil {
    return err
}

// Write value with TTL
if err := client.Set(ctx, "message", "hello from xredis", time.Minute); err != nil {
    return err
}

// Read value
value, ok, err := client.String(ctx, "message")
if err != nil {
    return err
}
if !ok {
    return xredis.ErrKeyNotFound
}

fmt.Println(value) // Outputs: hello from xredis
```
<!-- @formatter:on -->

## Clients and topologies

`xredis` provides dedicated constructors for each supported Redis topology, with a specialized configuration struct for
its connection and routing settings.

| Topology                           | Constructor                | Configuration    |
| :--------------------------------- | :------------------------- | :--------------- |
| **Standalone Redis**               | `NewClient`                | `ClientConfig`   |
| **Redis Cluster**                  | `NewClusterClient`         | `ClusterConfig`  |
| **Redis Sentinel / Failover**      | `NewFailoverClient`        | `FailoverConfig` |
| **Redis Sentinel with replica routing**        | `NewFailoverClusterClient` | `FailoverConfig` |
| **Client-side sharding with Ring** | `NewRing`                  | `RingConfig`     |

### Configuration capabilities

Configuration structs are plain Go structs and can be loaded with any configuration library. For a complete
environment-based setup, see [examples/env](examples/env).

Configuration structs and constructor options cover:

* **Connectivity and pools** — custom dialers, connection pooling, and routing strategies.
* **Security and authentication** — ACL credentials, dynamic credential providers, TLS, and mTLS.
* **Protocol and lifecycle** — RESP protocol selection, client identity, and hooks.
* **Data encoding** — configurable codecs for structured values.
* **Resilience** — retries, backoff policies, and fine-grained operation timeouts.
* **Observability** — OpenTelemetry tracing and custom metric labels.

### Cluster and sharding considerations

When using `xredis` with Redis Cluster or Redis Ring, keep the following topology-specific behaviors in mind:

* **Single-key operations** — compare-and-swap, compare-and-delete, lease locks, and rate-limit decisions operate on a
  single Redis key and do not require cross-slot coordination.
* **Fenced locks** — fenced locks use both a lock key and a counter key. In Redis Cluster, both keys must map to the
  same hash slot. Use matching hash tags, such as `lock:{order:42}` and `fence:{order:42}`.
* **Bulk helpers** — helpers such as `DeleteMany` and `UnlinkMany` use independent single-key commands where required,
  avoiding multi-key `CROSSSLOT` errors. Large inputs should still be divided into reasonable batches.
* **Topology-wide scans** — Redis Cluster scan cursors are node-local. The topology-wide scan helpers iterate over each
  master node independently. Redis Ring scans iterate over each live shard.
* **Raw client access** — commands executed through `Client.Raw()` bypass the higher-level topology-aware helpers.
  Multi-key commands must follow the normal Redis Cluster hash-slot rules.

### Example

The following example initializes a Redis Cluster client with a set of startup node addresses.

<!-- @formatter:off -->
```go
config := &xredis.ClusterConfig{
    Addrs: []string{
        "localhost:7000",
        "localhost:7001",
        "localhost:7002",
    },
}

client, err := xredis.NewClusterClient(
    xredis.WithClusterConfig(config),
)
if err != nil {
    log.Fatalf("create Redis Cluster client: %v", err)
}
```
<!-- @formatter:on -->

## Values and encoding

`xredis` supports both native Redis scalar values and structured Go values encoded through a configurable codec.

### Scalar values

Supported scalar types are passed directly to `go-redis` for native encoding and scanning:

<!-- @formatter:off -->
```go
if err := client.Set(ctx, "counter", 42, time.Minute); err != nil {
    log.Fatalf("set counter: %v", err)
}

counter, ok, err := client.Int64(ctx, "counter")
if err != nil {
    log.Fatalf("get counter: %v", err)
}

if !ok {
    log.Println("counter not found")
    return
}

fmt.Printf("Counter: %d\n", counter)
```
<!-- @formatter:on -->

Available scalar readers include `String`, `Bytes`, `Bool`, `Int`, `Int64`, `Uint64`, and `Float64`.

Additional command helpers include `SetNX`, `SetXX`, `GetDel`, `GetEx`, `Incr`, `Decr`, `Exists`, and `Delete`.

### Codec-backed values

Structured values are encoded through the client-level `Codec`. JSON is used by default.

<!-- @formatter:off -->
```go
type User struct {
    ID    string `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

// Save a structured object
user := User{ID: "42", Name: "Ada Lovelace", Email: "ada@example.com"}

if err := client.SetStruct(ctx, "user:42", user, 24*time.Hour); err != nil {
    log.Fatalf("store user: %v", err)
}

// Retrieve the object back
var loadedUser User

ok, err := client.GetStruct(ctx, "user:42", &loadedUser)
if err != nil {
    log.Fatalf("get user: %v", err)
}

if !ok {
    log.Println("user not found")
    return
}

fmt.Printf("Loaded %s <%s>\n", loadedUser.Name, loadedUser.Email)
```
<!-- @formatter:on -->

To use another serialization format, such as MessagePack or Protobuf, configure a custom codec when creating the client:

<!-- @formatter:off -->
```go
client, err := xredis.NewClient(
    xredis.WithClientConfig(cfg),
    xredis.WithCodec(customCodec),
)
```
<!-- @formatter:on -->

> [!NOTE]
> `SetStruct` and `GetStruct` store codec-backed Redis string values without revision metadata. For optimistic
> concurrency on structured values, use `VersionedStore[T]`.

### Redis hashes

`HSet` supports flat field-value pairs, slices, maps, structs, and pointers to structs. It can also apply an expiration
TTL to the hash key. Struct field names are configured using standard `redis` tags.

`HGetAll` can scan the resulting hash back into a struct:

<!-- @formatter:off -->
```go
type UserHash struct {
    Name   string `redis:"name"`
    Active bool   `redis:"active"`
}

user := UserHash{Name: "Grace Hopper", Active: true}

// Store the struct as a Redis hash with a TTL.
if err := client.HSet(ctx, "user:42", time.Hour, user); err != nil {
    log.Fatalf("set user hash: %v", err)
}

var loadedUser UserHash

// Read the hash fields back into a struct.
ok, err := client.HGetAll(ctx, "user:42", &loadedUser)
if err != nil {
    log.Fatalf("get user hash: %v", err)
}

if !ok {
    log.Println("user hash not found")
    return
}

fmt.Printf("Loaded %s (active: %t)\n", loadedUser.Name, loadedUser.Active)
```
<!-- @formatter:on -->

## Typed cache

`Cache[T]` implements a typed cache-aside workflow with TTL jitter, negative caching, and loader deduplication for
concurrent misses.

<!-- @formatter:off -->
```go
// Create a typed cache for User values.
cache, err := xredis.NewCache[User](
    client,
    xredis.WithCachePrefix("cache:user:"),
    xredis.WithCacheTTL(10*time.Minute),
    xredis.WithCacheJitter(15*time.Second),
    xredis.WithCacheNegativeTTL(30*time.Second),
)
if err != nil {
    log.Fatalf("create user cache: %v", err)
}

// Read the user from Redis or load it from the database on a cache miss.
// Concurrent misses for the same key share one in-flight loader execution.
user, err := cache.GetOrLoad(ctx, "42", func(ctx context.Context) (User, error) {
    return db.FetchUserByID(ctx, "42")
})
if err != nil {
    log.Fatalf("get user: %v", err)
}

fmt.Printf("Loaded user: %s\n", user.Name)
```
<!-- @formatter:on -->

### Core behaviors

* **Singleflight deduplication** — concurrent misses for the same key within one cache instance share a single loader
  execution, reducing duplicate requests to upstream data sources.
* **Negative caching** — `WithCacheNegativeTTL` caches not-found results for a limited time. Domain-specific errors can
  be recognized through `WithCacheNotFound`.
* **Interoperable positive values** — positive entries are stored without an internal metadata envelope and remain
  readable through regular Redis commands.
* **Type-aware encoding** — supported Redis scalar types use native `go-redis` encoding and scanning, while structured
  and user-defined values use the configured cache `Codec`.
* **Typed API** — `Get`, `Set`, and `GetOrLoad` operate on the cache type `T`, and loaders must return the same type.
  Interface cache types such as `Cache[any]` are rejected when the cache is created.

### Custom negative markers

Negative cache entries are represented by a reserved byte sequence. The default marker is `[]byte{0}`.

Use `WithCacheNegativeMarker` when the default marker may also occur as a valid cached value:

<!-- @formatter:off -->
```go
cache, err := xredis.NewCache[User](
    client,
    xredis.WithCacheNegativeTTL(30*time.Second),
    xredis.WithCacheNegativeMarker([]byte("\x00xredis:not-found\xff")),
)
```
<!-- @formatter:on -->

> [!WARNING]
> Cache instances sharing the same keyspace must use the same negative marker. When changing the marker, remove existing
> negative entries or use a new cache prefix.

## Atomic compare operations

`xredis` provides atomic compare-and-swap (CAS) and compare-and-delete (CAD) operations for raw Redis string values,
individual hash fields, and structured values. The operations are executed atomically in Redis using server-side Lua
scripts.

### Raw values

`CompareAndSwap` and `CompareAndDelete` compare the exact Redis string representation of a value before modifying or
removing it:

<!-- @formatter:off -->
```go
// Update the status only if it is still "processing".
swapped, err := client.CompareAndSwap(
    ctx,
    "order:42:status",
    "processing",
    "completed",
    xredis.KeepTTL,
)
if err != nil {
    return err
}
if !swapped {
    return errors.New("status changed concurrently")
}

// Delete the key only if its current value is still "completed".
deleted, err := client.CompareAndDelete(
    ctx,
    "order:42:status",
    "completed",
)
if err != nil {
    return err
}
if !deleted {
    return errors.New("status changed before deletion")
}
```
<!-- @formatter:on -->

> [!NOTE]
> Raw compare operations use the standard `go-redis` argument encoding and compare values byte-for-byte. The `expected`
> value must use the same representation as the value originally stored in Redis.

### Hash fields

`HCompareAndSwap` and `HCompareAndDelete` atomically compare and modify an individual Redis hash field:

<!-- @formatter:off -->
```go
// Update the "status" field only if it is still "processing".
swapped, err := client.HCompareAndSwap(
    ctx,
    "order:42",
    "status",
    "processing",
    "completed",
)
if err != nil {
    return err
}
if !swapped {
    return errors.New("status changed concurrently")
}

// Delete the "status" field only if it is still "completed".
deleted, err := client.HCompareAndDelete(
    ctx,
    "order:42",
    "status",
    "completed",
)
if err != nil {
    return err
}
if !deleted {
    return errors.New("status changed before deletion")
}
```
<!-- @formatter:on -->

> [!NOTE]
> Hash-field compare operations preserve the existing expiration of the hash key. If `HCompareAndDelete` removes the
> last remaining field, Redis removes the hash key.

### Versioned structured values

`VersionedStore[T]` provides revision-based optimistic concurrency control for structured values. Instead of sending
the previous encoded value back to Redis for comparison, update and delete operations validate a compact opaque
revision. A successful update still transmits the new encoded value.

#### Storage schema

Each versioned object is stored as a single Redis hash:

| Field | Representation | Description |
| :--- | :--- | :--- |
| `value` | Binary-safe string | Encoded representation of `T` |
| `revision` | String | Opaque optimistic-concurrency token |

#### Example

The following example creates a versioned order, reads its current revision, updates it conditionally, and deletes it
only if the revision still matches.

<!-- @formatter:off -->
```go
// Initialize a typed versioned store.
store, err := xredis.NewVersionedStore[Order](
	client,
	xredis.WithVersionedStorePrefix("versioned:order:"),
)
if err != nil {
	return err
}

// Create the object only if the Redis key does not exist.
revision, created, err := store.SetIfAbsent(
	ctx,
	"42",
	Order{
		ID:     "42",
		Status: "processing",
	},
	time.Hour,
)
if err != nil {
	return err
}
if !created {
	return errors.New("order already exists")
}

fmt.Println("created revision:", revision)

// Read the current value and revision.
entry, ok, err := store.Get(ctx, "42")
if err != nil {
	return err
}
if !ok {
	return xredis.ErrKeyNotFound
}

// Update the value locally.
updated := entry.Value
updated.Status = "completed"

// Replace the value only if the revision remains unchanged.
newRevision, swapped, err := store.CompareAndSwap(
	ctx,
	"42",
	entry.Revision,
	updated,
	xredis.KeepTTL,
)
if err != nil {
	return err
}
if !swapped {
	return errors.New("order changed concurrently")
}

// Delete the object only if the revision still matches.
deleted, err := store.CompareAndDelete(
	ctx,
	"42",
	newRevision,
)
if err != nil {
	return err
}
if !deleted {
	return errors.New("order changed before deletion")
}
```
<!-- @formatter:on -->

> [!IMPORTANT]
> `VersionedStore[T]` uses a dedicated Redis hash representation. Keys managed by a versioned store must not be modified
> through Redis string-value methods such as `Set` or `SetStruct`. Store instances sharing the same keyspace must use
> the same value type and `Codec`.

### Expiration management

`SetIfAbsent` and `CompareAndSwap` manage the expiration of the versioned object as follows:

| Value             | `SetIfAbsent`                     | `CompareAndSwap`                                             |
| :---------------- | :--------------------------- | :----------------------------------------------------------- |
| `xredis.KeepTTL`  | Returns `ErrInvalidTTL`      | Preserves the existing expiration                            |
| `0`               | Creates a persistent key     | Removes the existing expiration and makes the key persistent |
| Positive duration | Applies the given expiration | Replaces the existing expiration                             |

For a complete runnable example, see [examples/cas](examples/cas).

## Distributed locks

`xredis` provides two primitives for distributed coordination: token-based **lease locks** and **fenced locks** with
monotonically increasing fencing tokens. Lock operations use atomic Redis commands and server-side Lua scripts.

### Lease locks

Lease locks use an internally generated or application-provided owner token. `Unlock` and `Extend` succeed only while
Redis still stores the same token, preventing one client from modifying a lock that has expired and been acquired by
another client.

<!-- @formatter:off -->
```go
// Acquire a 30-second lease for the order.
lock, acquired, err := client.TryLock(ctx, "lock:order:42", 30*time.Second)
if err != nil {
    return fmt.Errorf("acquire order lock: %w", err)
}
if !acquired {
    return errors.New("order is already being processed")
}

// Release the lock using an independent timeout context.
defer func() {
    unlockCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    if err := lock.Unlock(unlockCtx); err != nil &&
        !errors.Is(err, xredis.ErrLockNotOwned) {
        log.Printf("release order lock: %v", err)
    }
}()

// Perform work protected by the lease.

// Extend the lease before it expires if more time is needed.
extended, err := lock.Extend(ctx, 30*time.Second)
if err != nil {
    return fmt.Errorf("extend order lock: %w", err)
}
if !extended {
    return errors.New("lock lease expired or ownership changed")
}

// Continue the protected operation.
```
<!-- @formatter:on -->

Use `TryLockWithToken` when token generation is managed by the application. Tokens must be unique for every independent
lock attempt.

### Fenced locks

Fenced locks combine a lease lock with a monotonically increasing fencing token. They protect against stale clients that
continue operating after their lease has expired, for example after a long pause or network delay.

<!-- @formatter:off -->
```go
// Acquire a 30-second fenced lease for the order.
lock, acquired, err := client.TryFencedLock(
    ctx,
    "lock:{order:42}",
    "fence:{order:42}",
    30*time.Second,
)
if err != nil {
    return fmt.Errorf("acquire fenced lock: %w", err)
}
if !acquired {
    return errors.New("order is already being processed")
}

// Release the lease using an independent timeout context.
defer func() {
    unlockCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    if err := lock.Unlock(unlockCtx); err != nil &&
        !errors.Is(err, xredis.ErrLockNotOwned) {
        log.Printf("release fenced lock: %v", err)
    }
}()

// Include the fencing token in every protected write.
// The downstream storage must reject writes with an older token.
token := lock.FencingToken()
```
<!-- @formatter:on -->

The fencing token increases monotonically for each successful acquisition using the same fencing counter key.

#### Critical fencing invariants

* **Downstream enforcement** — the protected resource must persist the latest accepted fencing token and reject
  operations carrying a smaller token. The Redis lock alone cannot enforce this invariant.
* **Counter lifetime** — fencing counters do not expire by default, because resetting a counter can make stale tokens
  valid again. Use `WithFencingCounterTTL` only for resources with a bounded lifecycle and a safe retention period.

> [!WARNING]
> In Redis Cluster, the lock key and fencing counter key must map to the same hash slot. Use matching Redis hash tags,
> such as the shared `{order:42}` tag in the example above.

## Rate limiter

`RateLimiter` provides distributed rate limiting with atomic server-side decisions. The algorithm is selected
independently for each request.

<!-- @formatter:off -->
```go
// Create a rate limiter with a shared Redis key prefix.
limiter, err := client.RateLimiter(
    xredis.WithRateLimiterPrefix("rate_limit:"),
)
if err != nil {
    return fmt.Errorf("create rate limiter: %w", err)
}

// Allow 10 requests per second with a burst capacity of 20.
decision, err := limiter.AllowTokenBucket(
    ctx,
    "user:42:api",
    xredis.TokenBucketRateLimit{
        Limit:  10,
        Window: time.Second,
        Burst:  20,
    },
)
if err != nil {
    return fmt.Errorf("check rate limit: %w", err)
}

if !decision.Allowed {
    return fmt.Errorf(
        "rate limit exceeded; retry after %s",
        decision.RetryAfter,
    )
}

// Continue processing the request.
```
<!-- @formatter:on -->

### Available algorithms

* **Fixed window** (`Allow` / `AllowFixedWindow`) — simple and memory-efficient, but may allow bursts around window
  boundaries.
* **Sliding window** (`AllowSlidingWindow`) — tracks accepted requests in a Redis sorted set and provides more accurate
  limiting within the active window.
* **Token bucket** (`AllowTokenBucket`) — supports controlled bursts with gradual token refill.

Each algorithm uses a different Redis data structure. Use separate keys when applying multiple algorithms to the same
subject.

> [!NOTE]
> Every rate-limit decision is executed atomically using a single Redis key. The algorithms are therefore compatible
> with Redis Cluster without requiring multi-key hash-slot coordination.

## Pipelines and topology-wide scans

`xredis` provides pipeline helpers for bulk operations and topology-aware scan helpers for standalone Redis, Cluster,
and Ring clients.

### Pipeline helpers

Pipeline helpers execute independent single-key commands in batches:

<!-- @formatter:off -->
```go
// Store multiple values in a single pipeline execution.
err := client.SetMany(ctx, []xredis.SetItem{
    {
        Key:        "session:101",
        Value:      "user:42",
        Expiration: time.Hour,
    },
    {
        Key:        "session:102",
        Value:      true,
        Expiration: 30 * time.Minute,
    },
})
if err != nil {
    return fmt.Errorf("store sessions: %w", err)
}
```
<!-- @formatter:on -->

`SetMany` and `SetStructMany` batch string-value writes, `HSetMany` batches hash writes, and `DeleteMany` and
`UnlinkMany` batch key removal.

> [!IMPORTANT]
> For Redis Cluster and Ring clients, `DeleteMany` and `UnlinkMany` use pipelined single-key commands to avoid multi-key
> cross-slot errors. Large inputs should be split into reasonable batches at the call site.

### Topology-wide scans

Topology-wide scan helpers coordinate iteration across Redis nodes and support both per-key and per-batch handlers:

<!-- @formatter:off -->
```go
// Process matching keys in batches across all topology nodes.
err := client.ScanEachBatch(
    ctx,
    xredis.ScanOptions{
        Match: "cache:user:*",
        Count: 500,
    },
    func(ctx context.Context, keys []string) error {
        return processCacheKeys(ctx, keys)
    },
)
if err != nil {
    return fmt.Errorf("scan user cache keys: %w", err)
}
```
<!-- @formatter:on -->

Available scan helpers include:

* `Scan` — reads one cursor page.
* `ScanAll` — collects all matching keys.
* `ScanEach` — invokes a handler for each key.
* `ScanEachBatch` — invokes a handler for each page.
* `ScanDelete` and `ScanUnlink` — remove matching keys.

> [!NOTE]
> Redis `SCAN` provides weakly consistent iteration. Keys may be added, removed, or returned more than once while a scan
> is in progress. Handlers should therefore be idempotent.
>
> `Count` is a work-size hint to Redis, not a guaranteed batch size. Topology-wide scan and removal operations are not
> atomic.

## Observability

`xredis` integrates with OpenTelemetry for Redis metrics and distributed tracing.

### Metrics

Initialize observability once before creating Redis clients. Call the returned shutdown function during application
shutdown:

<!-- @formatter:off -->
```go
// Initialize global Redis metrics instrumentation.
shutdownMetrics, err := xredis.InitObservability(
    xredis.WithMeterProvider(meterProvider),
)
if err != nil {
    log.Fatalf("initialize Redis metrics: %v", err)
}

defer func() {
    if err := shutdownMetrics(); err != nil {
        log.Printf("shutdown Redis metrics: %v", err)
    }
}()

// Create a client with service-level metric labels.
client, err := xredis.NewClient(
    xredis.WithAddr("localhost:6379"),
    xredis.WithMetricLabel("service", "orders-api"),
    xredis.WithMetricLabel("environment", "production"),
)
if err != nil {
    log.Fatalf("create Redis client: %v", err)
}
defer client.Close()
```
<!-- @formatter:on -->

`InitObservability` enables native `go-redis` metrics through `redisotel-native` together with `xredis` wrapper-level
metrics.

Native metric groups, command filters, histogram aggregation, and histogram buckets are configured through
`ObservabilityOption` values. Additional bounded labels can be attached to an individual client with `WithMetricLabel`.

> [!WARNING]
> Metric label values should have low and bounded cardinality. Avoid identifiers such as Redis keys, user IDs, request
> IDs, or other values that can create an unbounded number of time series.

Prometheus exporters expose the wrapper-level OpenTelemetry instruments with the following names:

| Prometheus metric                              | Type      | Description                                                 |
| :--------------------------------------------- | :-------- | :---------------------------------------------------------- |
| `redis_client_cache_requests_total`            | Counter   | Counts cache lookups by operation and result.               |
| `redis_client_cache_loader_duration_seconds`   | Histogram | Measures cache loader execution duration.                   |
| `redis_client_cache_singleflight_shared_total` | Counter   | Counts requests that received a shared singleflight result. |
| `redis_client_lock_operations_total`           | Counter   | Counts lease and fenced lock operations by outcome.         |
| `redis_client_rate_limiter_decisions_total`    | Counter   | Counts rate-limit decisions by algorithm and outcome.       |
| `redis_client_rate_limiter_duration_seconds`   | Histogram | Measures rate-limit decision duration.                      |

### Metric labels

The following labels are exposed by the wrapper-level `xredis` metrics and can be used to filter, group, and aggregate
telemetry data:

| Label                                 | Values                                           | Description                                   |
| :------------------------------------ | :----------------------------------------------- | :-------------------------------------------- |
| `redis_client_cache_operation`        | `get`, `get_or_load`                             | Cache operation being performed               |
| `redis_client_cache_result`           | `hit`, `miss`, `negative_hit`, `error`           | Result of the cache lookup                    |
| `redis_client_cache_loader_outcome`   | `success`, `not_found`, `error`                  | Outcome of the cache loader execution         |
| `redis_client_lock_type`              | `lease`, `fenced`                                | Type of distributed lock                      |
| `redis_client_lock_operation`         | `acquire`, `extend`, `unlock`                    | Lock operation being performed                |
| `redis_client_lock_outcome`           | `success`, `contended`, `not_owned`, `error`     | Result of the lock operation                  |
| `redis_client_rate_limiter_algorithm` | `fixed_window`, `sliding_window`, `token_bucket` | Rate-limiting algorithm used for the decision |
| `redis_client_rate_limiter_outcome`   | `allowed`, `rejected`, `error`                   | Result of the rate-limit decision             |

### Tracing

Tracing is configured separately for each Redis client through the `redisotel` integration:

<!-- @formatter:off -->
```go
client, err := xredis.NewClient(
    xredis.WithClientConfig(cfg),
    xredis.WithTracerProvider(tracerProvider),
    xredis.WithTracingDBStatement(false),
    xredis.WithTracingCallerEnabled(true),
)
```
<!-- @formatter:on -->

Additional tracing options support custom span attributes, DB system attributes, command and pipeline filters, dial
filters, and caller information.

> [!WARNING]
> `WithTracingDBStatement(true)` can include Redis command contents in spans. Avoid enabling it when commands may
> contain sensitive keys, values, credentials, or personally identifiable information.

For a complete OTLP tracing setup with HTTP parent spans and Jaeger, see [examples/otel](examples/otel).

## License

This project is licensed under the [MIT License](LICENSE).
