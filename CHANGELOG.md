# Changelog

## v0.3.1

This release improves typed cache decoding performance while keeping the public API unchanged.

### Changed

* **Cache decoding** — reduced reflection overhead for pointer-based cache values while retaining the existing
  allocation behavior across Redis-native and codec-backed decoding paths.

## v0.3.0

This release adopts Go 1.27, adds generic client factory methods, and refreshes the core Redis and observability
dependencies.

### Added

* **Generic client factories** — added `Client.Cache[T]` and `Client.VersionedStore[T]` for creating typed components
  directly from an existing client, alongside the existing `NewCache[T]` and `NewVersionedStore[T]` constructors.

### Changed

* **Go 1.27** — raised the minimum supported Go version to 1.27 and adopted generic methods in the public client API.
* **Dependency updates** — updated `go-redis`, OpenTelemetry, Prometheus, and related dependencies to their latest
  compatible releases.

## v0.2.1

Initial release of `xredis`, providing an opinionated `go-redis` wrapper with application-level reliability patterns,
topology-aware operations, and OpenTelemetry observability.

### Added

* **Multiple topologies** — dedicated constructors and configuration types for standalone Redis, Redis Cluster,
  Sentinel/failover with replica routing, and client-side sharded Ring deployments.
* **Native and structured values** — native `go-redis` encoding and scanning for standard scalar types, with
  configurable `Codec` support for structured values.
* **Command helpers** — typed value readers, conditional writes, atomic counters, existence checks, deletion helpers,
  and struct-to-hash mapping.
* **Typed cache-aside** — generic `Cache[T]` workflows with TTL jitter, negative caching, configurable not-found
  detection, custom negative markers, and singleflight deduplication for concurrent misses.
* **Atomic compare operations** — Lua-backed compare-and-swap (CAS) and compare-and-delete (CAD) operations for exact
  Redis string values and individual hash fields.
* **Versioned structured values** — generic `VersionedStore[T]` workflows with opaque revision tokens, atomic
  initialization through `SetIfAbsent`, optimistic updates, conditional deletion, and configurable expiration.
* **Distributed locks** — token-based lease locks with ownership-safe extension and unlock, together with fenced locks
  using monotonically increasing fencing tokens.
* **Distributed rate limiting** — atomic fixed window, sliding window, and token bucket algorithms implemented with
  server-side Lua scripts.
* **Bulk operations and pipelines** — pipelined helpers for scalar values, structured values, hashes, deletion, and
  unlink operations.
* **Topology-wide scans** — cursor-based iteration across Redis Cluster masters and Redis Ring shards, with type
  filtering and per-key or per-batch handlers.
* **OpenTelemetry integration** — distributed command tracing through `redisotel`, native `go-redis` metrics through
  `redisotel-native`, and wrapper-level metrics for caches, locks, and rate limiters.
* **Testing and examples** — Redis integration tests and runnable examples for commands, caching, compare operations,
  locks, rate limiting, pipelines, scans, configuration, and OpenTelemetry tracing with Jaeger.