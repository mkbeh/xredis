# Examples

This directory contains runnable examples demonstrating the main features and usage patterns of `xredis`.

| Example                        | Demonstrates                                                                                            |
|:-------------------------------|:--------------------------------------------------------------------------------------------------------|
| [`commands`](commands)         | Native Redis scalar values, structured encoding, hashes, counters, and command helpers                  |
| [`cache`](cache)               | Typed `Cache[T]` workflows, singleflight deduplication, TTL jitter, negative caching, and cache metrics |
| [`cas`](cas)                   | Atomic compare-and-swap and compare-and-delete operations for raw and structured values                 |
| [`locks`](locks)               | Distributed coordination with lease locks and fenced locks                                              |
| [`rate_limiter`](rate_limiter) | Distributed rate limiting with fixed window, sliding window, and token bucket algorithms                |
| [`pipeline`](pipeline)         | Batched writes, hash updates, delete, and unlink helpers                                                |
| [`scan`](scan)                 | Cursor scans, topology-wide iteration, key-type filters, delete, and unlink operations                  |
| [`env`](env)                   | Loading client configuration from environment variables                                                 |
| [`otel`](otel)                 | Exporting OpenTelemetry traces through OTLP to Jaeger                                                   |

## Running the examples

The examples use Docker Compose to start Redis and any required supporting services.

From the `examples` directory, start the topology required by the example:

```bash
docker compose --profile standalone up -d
```

Then run the example from its directory:

```bash
cd cache
go run .
```

> [!NOTE]
> Some examples require a different Docker Compose profile or additional services. Refer to the README in the
> corresponding example directory for the exact startup command and configuration.
