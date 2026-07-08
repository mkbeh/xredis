# Redis CAS/CAD REST API

This example shows how to use compare-and-swap and compare-and-delete operations with `xredis`.

**This example demonstrates:**

* Raw value CAS with `CompareAndSwap`
* Raw value CAD with `CompareAndDelete`
* Struct value CAS with `CompareAndSwapStruct`
* Struct value CAD with `CompareAndDeleteStruct`
* Stale expected value rejection

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
Database alias: redis-cas-example
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
go run ./examples/cas
```

The HTTP server starts on:

```text
localhost:8080
```

## Health check

```shell
curl 'localhost:8080/healthz'
```

## Raw value CAS flow

Seed a raw status value:

```shell
curl -X POST 'localhost:8080/statuses/42/seed'
```

Complete it using CAS:

```shell
curl -X POST 'localhost:8080/statuses/42/complete'
```

Expected result:

```text
HTTP 200
{"swapped":true,...}
```

Read the raw status:

```shell
curl 'localhost:8080/statuses/42'
```

Run the same CAS again:

```shell
curl -X POST 'localhost:8080/statuses/42/complete'
```

Expected result:

```text
HTTP 409
{"error":"compare condition failed"}
```

The second call fails because the current value is no longer `processing`.

## Raw value CAD flow

Seed a raw status value:

```shell
curl -X POST 'localhost:8080/statuses/7/seed'
```

Delete it only if the current value is `processing`:

```shell
curl -X DELETE 'localhost:8080/statuses/7/processing'
```

Expected result:

```text
HTTP 200
{"deleted":true,...}
```

## Stale raw value check

This endpoint seeds `processing`, swaps it to `cancelled`, then tries to swap the stale expected value `processing` to `completed`.

```shell
curl -X POST 'localhost:8080/statuses/42/stale'
```

Expected result:

```json
{
  "first_swapped": true,
  "stale_swapped": false,
  "current_status": "cancelled"
}
```

## Struct value CAS flow

Seed an encoded order value with `SetStruct`:

```shell
curl -X POST 'localhost:8080/orders/42/seed'
```

Complete it using `CompareAndSwapStruct`:

```shell
curl -X POST 'localhost:8080/orders/42/complete'
```

Expected result:

```text
HTTP 200
{"swapped":true,...}
```

Read the order:

```shell
curl 'localhost:8080/orders/42'
```

## Struct value CAD flow

Seed an encoded order value:

```shell
curl -X POST 'localhost:8080/orders/7/seed'
```

Delete it only if the current encoded value still matches the value read by the handler:

```shell
curl -X DELETE 'localhost:8080/orders/7/current'
```

Expected result:

```text
HTTP 200
{"deleted":true,...}
```

## Stale struct value check

This endpoint seeds an order, swaps it from `processing` to `cancelled`, then tries to swap the stale old order to `completed`.

```shell
curl -X POST 'localhost:8080/orders/42/stale'
```

Expected result:

```json
{
  "first_swapped": true,
  "stale_swapped": false,
  "current_order": {
    "status": "cancelled"
  }
}
```

## Cleanup

Deletes known sample keys.

```shell
curl -X DELETE 'localhost:8080/sample'
```

## Redis Cluster note

CAS/CAD operations in this example use one Redis key per Lua script call.

For Redis Cluster, single-key Lua scripts are routed to the node that owns that key, so no hash tag is required for these operations.

## Stop services

From the repository root:

```shell
docker compose -f examples/docker-compose.yml --profile standalone down --remove-orphans -v
```

Or from this example directory:

```shell
docker compose -f ../docker-compose.yml --profile standalone down --remove-orphans -v
```