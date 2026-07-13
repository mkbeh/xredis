# Redis Rate Limiter REST API

This example shows how to use Redis-backed application rate limiting with `xredis`.

**This example demonstrates:**

* Fixed-window rate limiting with `AllowFixedWindow`
* Default `Allow`, which uses fixed-window rate limiting
* Sliding-window rate limiting with `AllowSlidingWindow`
* Token-bucket rate limiting with `AllowTokenBucket`
* HTTP 429 responses with `Retry-After` and `X-RateLimit-*` headers

## Configuration

```text
REDIS_ADDR=localhost:6379
HTTP_ADDR=localhost:8080
```

## Local Redis setup

Examples can use the local Redis setup from `examples/docker-compose.yml`.

From the repository root:

```shell
docker compose -f examples/docker-compose.yml --profile standalone up -d
```

Or from this example directory:

```shell
docker compose -f ../docker-compose.yml --profile standalone up -d
```

Redis is available at:

```text
localhost:6379
```

RedisInsight is available at:

```text
http://localhost:5540
```

Inside RedisInsight, add a Redis database with:

```text
Database alias: redis-rate-limiter-example
Host: redis-standalone
Port: 6379
Username: default
Password: empty
Database index: 0
```

## Run

From this directory:

```shell
go run .
```

Or from the repository root:

```shell
go run ./examples/rate_limiter
```

The HTTP server starts on:

```text
localhost:8080
```

## Metrics

Prometheus metrics are available at:

```shell
curl 'http://localhost:8080/metrics'
```

Useful rate limiter metrics for this example include:

```text
redis_client_rate_limiter_decisions_total
redis_client_rate_limiter_duration_seconds
```

Rate limit decisions:

```text
redis_client_rate_limiter_decisions_total{redis_client_rate_limiter_algorithm="fixed_window",redis_client_rate_limiter_outcome="allowed"}
redis_client_rate_limiter_decisions_total{redis_client_rate_limiter_algorithm="fixed_window",redis_client_rate_limiter_outcome="rejected"}
redis_client_rate_limiter_decisions_total{redis_client_rate_limiter_algorithm="sliding_window",redis_client_rate_limiter_outcome="allowed"}
redis_client_rate_limiter_decisions_total{redis_client_rate_limiter_algorithm="sliding_window",redis_client_rate_limiter_outcome="rejected"}
redis_client_rate_limiter_decisions_total{redis_client_rate_limiter_algorithm="token_bucket",redis_client_rate_limiter_outcome="allowed"}
redis_client_rate_limiter_decisions_total{redis_client_rate_limiter_algorithm="token_bucket",redis_client_rate_limiter_outcome="rejected"}
```

Check rate limit decision metrics:

```shell
curl -s 'http://localhost:8080/metrics'   | grep 'redis_client_rate_limiter_decisions_total'
```

Check rate limiter duration metrics:

```shell
curl -s 'http://localhost:8080/metrics'   | grep 'redis_client_rate_limiter_duration_seconds'
```

Check duration counts by algorithm and outcome:

```shell
curl -s 'http://localhost:8080/metrics'   | grep 'redis_client_rate_limiter_duration_seconds_count'
```

## Health check

```shell
curl 'localhost:8080/healthz'
```

## Default allow flow

`Allow` uses fixed-window rate limiting and is equivalent to `AllowFixedWindow`.

```shell
curl -i -X POST 'localhost:8080/allow/42'
```

## Fixed-window flow

This endpoint allows 5 requests per 30 seconds for one user key.

```shell
curl -i -X POST 'localhost:8080/fixed-window/42'
```

Run several requests:

```shell
seq 1 7 | xargs -I{} curl -i -s -X POST 'localhost:8080/fixed-window/42'
```

Expected result:

```text
first 5 requests -> HTTP 200
next requests    -> HTTP 429
```

The response contains rate-limit headers:

```text
X-RateLimit-Limit
X-RateLimit-Remaining
X-RateLimit-Reset
Retry-After
```

## Sliding-window flow

This endpoint allows 5 requests within the last 30 seconds.

```shell
curl -i -X POST 'localhost:8080/sliding-window/42'
```

Run several requests:

```shell
seq 1 7 | xargs -I{} curl -i -s -X POST 'localhost:8080/sliding-window/42'
```

Expected result:

```text
first 5 requests -> HTTP 200
next requests    -> HTTP 429
```

Sliding window is more accurate than fixed window, but stores one sorted-set entry per accepted request within the
window.

## Token-bucket flow

This endpoint refills 5 tokens per 30 seconds and allows bursts up to 10 requests.

```shell
curl -i -X POST 'localhost:8080/token-bucket/42'
```

Run several requests:

```shell
seq 1 12 | xargs -I{} curl -i -s -X POST 'localhost:8080/token-bucket/42'
```

Expected result:

```text
first requests up to burst -> HTTP 200
after bucket is empty      -> HTTP 429
```

Wait a few seconds and try again:

```shell
sleep 10
curl -i -X POST 'localhost:8080/token-bucket/42'
```

Some tokens should be refilled.

## Cleanup

Deletes known sample rate-limit keys with `DeleteMany`.

```shell
curl -X DELETE 'localhost:8080/sample'
```

## Redis Cluster note

Current rate-limit operations use one Redis key per Lua script call:

```text
fixed-window   -> one key
sliding-window -> one key
token-bucket   -> one key
```

So these operations are safe to use with Redis Cluster.

Avoid using a shared hash tag in the rate-limiter prefix, because that can route many user keys to the same cluster slot
and create a hot shard.

Good:

```text
xredis:rate_limiter:fixed:user:42
xredis:rate_limiter:fixed:user:43
```

Risky if used globally:

```text
xredis:rate_limiter:{rl}:fixed:user:42
xredis:rate_limiter:{rl}:fixed:user:43
```

## Stop services

From the repository root:

```shell
docker compose -f examples/docker-compose.yml --profile standalone down --remove-orphans -v
```

Or from this example directory:

```shell
docker compose -f ../docker-compose.yml --profile standalone down --remove-orphans -v
```