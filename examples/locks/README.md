# Redis Locks REST API

This example shows how to use Redis lease locks and fenced locks with `xredis`.

**This example demonstrates:**

* Redis lease lock with `TryLock`
* Safe `Unlock` with owner token check
* Lock TTL extension with `Extend`
* Fenced lock with monotonically increasing fencing token
* Fencing counter retention with `WithFencingCounterTTL`
* Resource-side stale fencing token rejection

## Configuration

```text
REDIS_ADDR=localhost:6379
HTTP_ADDR=localhost:8080
````

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

From this directory:

```shell
go run .
```

Or from the repository root:

```shell
go run ./examples/locks
```

The HTTP server starts on:

```text
localhost:8080
```

## Health check

```shell
curl 'localhost:8080/healthz'
```

## Simple lock flow

This endpoint acquires a Redis lease lock, processes an order, and unlocks it.

```shell
curl -X POST 'localhost:8080/locks/42/work'
```

Expected result:

```text
HTTP 200
order is processed while lock is held
```

Run concurrent requests:

```shell
seq 1 20 | xargs -P20 -I{} curl -s -X POST 'localhost:8080/locks/42/work'
```

Some requests may return:

```text
HTTP 409
{"error":"lock not acquired"}
```

## Extend lock flow

This endpoint acquires a short lock, extends it, then processes an order.

```shell
curl -X POST 'localhost:8080/locks/42/extend'
```

Expected result:

```text
HTTP 200
{"extended":true,...}
```

## Fenced lock flow

This endpoint acquires a fenced lock and passes the fencing token to the protected resource.

```shell
curl -X POST 'localhost:8080/orders/42/process'
```

Expected result:

```text
HTTP 200
response contains fencing_token
```

Read the order:

```shell
curl 'localhost:8080/orders/42'
```

The order contains the last accepted fencing token.

## Stale fencing token check

This endpoint demonstrates that an old fencing token is rejected by the protected resource.

```shell
curl -X POST 'localhost:8080/orders/42/stale'
```

Expected result:

```text
HTTP 200
{"stale_token_rejected":true,...}
```

## Cleanup

Deletes known sample lock and fencing keys and resets the in-memory repository.

```shell
curl -X DELETE 'localhost:8080/sample'
```

## Redis Cluster note

Fenced lock uses two Redis keys in one Lua script:

```text
lock key
fencing counter key
```

For Redis Cluster, both keys must belong to the same hash slot and therefore be handled by the same cluster master node.

This example uses hash tags:

```text
xredis:locks:{order:42}
xredis:fencing:{order:42}
```

Both keys use the same hash tag: `{order:42}`.

## Stop services

From the repository root:

```shell
docker compose -f examples/docker-compose.yml --profile standalone down --remove-orphans -v
```

Or from this example directory:

```shell
docker compose -f ../docker-compose.yml --profile standalone down --remove-orphans -v
```