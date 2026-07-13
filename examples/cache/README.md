# Typed Cache REST API

This example shows how to use `xredis.Cache[T]` in a simple REST API service.

**This example demonstrates:**

* Standalone Redis client configuration
* Redis health check with `PING`
* Typed cache `Get`, `Set`, `Delete`, and `GetOrLoad`
* Default codec encoding for structured values
* Singleflight for concurrent cache misses
* TTL jitter
* Negative caching for not-found results
* Native Redis and cache-level OpenTelemetry metrics

## Configuration

Configure Redis connection using environment variables:

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
Database alias: redis-cache-example
Host: redis-standalone
Port: 6379
Username: default
Password: empty
Database index: 0
```

## Run

From this example directory:

```shell
go run .
```

Or from the repository root:

```shell
go run ./examples/cache
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

Useful cache metrics for this example include:

```text
redis_client_cache_requests_total
redis_client_cache_loader_duration_seconds
redis_client_cache_singleflight_shared_total
```

Cache lookup results:

```text
redis_client_cache_requests_total{redis_client_cache_operation="get",redis_client_cache_result="hit"}
redis_client_cache_requests_total{redis_client_cache_operation="get",redis_client_cache_result="miss"}
redis_client_cache_requests_total{redis_client_cache_operation="get_or_load",redis_client_cache_result="hit"}
redis_client_cache_requests_total{redis_client_cache_operation="get_or_load",redis_client_cache_result="miss"}
redis_client_cache_requests_total{redis_client_cache_operation="get_or_load",redis_client_cache_result="negative_hit"}
redis_client_cache_requests_total{redis_client_cache_operation="get_or_load",redis_client_cache_result="error"}
```

Check cache request metrics:

```shell
curl -s 'http://localhost:8080/metrics' | grep 'redis_client_cache_requests_total'
```

Check loader duration metrics:

```shell
curl -s 'http://localhost:8080/metrics' | grep 'redis_client_cache_loader_duration_seconds'
```

Check loader outcomes:

```shell
curl -s 'http://localhost:8080/metrics' \
  | grep 'redis_client_cache_loader_duration_seconds_count'
```

Check requests sharing a singleflight result:

```shell
curl -s 'http://localhost:8080/metrics' \
  | grep 'redis_client_cache_singleflight_shared_total'
```

## Health check

Checks Redis availability using `PING`.

```shell
curl 'localhost:8080/healthz'
```

Expected result:

```text
HTTP 200
{"status":"ok"}
```

## API flow

This example is easier to check as a sequence:

```text
1. Reset sample state
2. Try to read a user from cache only
3. Load the user through GetOrLoad
4. Read the same user from cache again
5. Check repository load stats
```

## 1. Reset sample state

Deletes the known sample cache keys and resets the in-memory repository and load counters.

```shell
curl -X DELETE 'localhost:8080/sample'
```

Expected result:

```text
HTTP 200
{"status":"reset"}
```

## 2. Read from cache only

This endpoint uses `Cache.Get`.

```shell
curl 'localhost:8080/users/42'
```

Expected result before loading:

```text
HTTP 404
{"error":"key not found"}
```

At this point Redis does not have the key:

```text
xredis:cache:user:42
```

## 3. Read through GetOrLoad

This endpoint uses `Cache.GetOrLoad`.

On cache miss, it calls the repository loader and stores the result in Redis using the cache codec.

```shell
curl 'localhost:8080/users/42/load'
```

Expected result:

```text
HTTP 200
{"key":"xredis:cache:user:42","source":"cache_or_loader","user":{"id":"42","name":"Ada Lovelace from repository","age":36,"active":true}}
```

The key is now visible in RedisInsight as:

```text
xredis:cache:user:42
```

## 4. Read from cache again

This endpoint uses `Cache.Get` again.

```shell
curl 'localhost:8080/users/42'
```

Expected result:

```text
HTTP 200
{"key":"xredis:cache:user:42","source":"cache","user":{"id":"42","name":"Ada Lovelace from repository","age":36,"active":true}}
```

This time the value is read from Redis and decoded by the cache codec.

## 5. Check repository load stats

Shows how many times the repository loader was called.

```shell
curl 'localhost:8080/stats'
```

Expected result:

```text
HTTP 200
{"repository_loads":{"42":1}}
```

Even though the user was requested twice, the repository was called only once.

## Write-through flow

Use `PUT /users/{id}` to store a user in both the repository and cache.

```shell
curl -X PUT 'localhost:8080/users/100' \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "Katherine Johnson",
    "age": 101,
    "active": true
  }'
```

Expected result:

```text
HTTP 200
{"key":"xredis:cache:user:100","source":"write_through","user":{"id":"100","name":"Katherine Johnson","age":101,"active":true}}
```

Read it from cache:

```shell
curl 'localhost:8080/users/100'
```

Expected result:

```text
HTTP 200
{"key":"xredis:cache:user:100","source":"cache","user":{"id":"100","name":"Katherine Johnson","age":101,"active":true}}
```

Delete it from the repository and cache:

```shell
curl -X DELETE 'localhost:8080/users/100'
```

Expected result:

```text
HTTP 200
{"key":"xredis:cache:user:100","status":"deleted"}
```

## Singleflight flow

This checks that concurrent `GetOrLoad` calls for the same key share one repository load.

Reset the example first:

```shell
curl -X DELETE 'localhost:8080/sample'
```

Run many concurrent requests for the same missing cache key:

```shell
seq 1 20 | xargs -P20 -I{} curl -s 'localhost:8080/users/7/load' >/dev/null
```

Check repository load stats:

```shell
curl 'localhost:8080/stats'
```

Expected result:

```text
HTTP 200
{"repository_loads":{"7":1}}
```

Only one repository load should happen because concurrent `GetOrLoad` calls are deduplicated by `singleflight`.

## Negative caching flow

This checks that not-found results are cached for a short TTL.

Reset the example first:

```shell
curl -X DELETE 'localhost:8080/sample'
```

Request a missing user:

```shell
curl 'localhost:8080/users/404/load'
```

Expected result:

```text
HTTP 404
{"error":"key not found"}
```

Request the same missing user again:

```shell
curl 'localhost:8080/users/404/load'
```

Expected result:

```text
HTTP 404
{"error":"key not found"}
```

Check repository load stats:

```shell
curl 'localhost:8080/stats'
```

Expected result:

```text
HTTP 200
{"repository_loads":{"404":1}}
```

The second request does not call the repository because the not-found result is cached.

The negative TTL is 30 seconds and the cache applies up to 5 seconds of jitter, so wait long enough for the entry to
expire:

```shell
sleep 36
curl 'localhost:8080/users/404/load'
curl 'localhost:8080/stats'
```

Expected results:

```text
HTTP 404
{"error":"key not found"}

HTTP 200
{"repository_loads":{"404":2}}
```

## Cleanup

Deletes the known sample cache keys for IDs `42`, `7`, `100`, and `404`, then resets the in-memory repository and load
counters.

```shell
curl -X DELETE 'localhost:8080/sample'
```

Expected result:

```text
HTTP 200
{"status":"reset"}
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