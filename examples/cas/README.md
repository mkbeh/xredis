# Redis CAS/CAD REST API

This example demonstrates atomic compare operations for raw Redis values and
revision-based optimistic concurrency for structured values.

**This example demonstrates:**

* Raw value CAS with `CompareAndSwap`
* Raw value CAD with `CompareAndDelete`
* TTL preservation with `KeepTTL`
* Structured value creation with `VersionedStore[T]`
* Revision-based CAS with `VersionedStore.CompareAndSwap`
* Revision-based CAD with `VersionedStore.CompareAndDelete`
* Stale value and stale revision rejection

## Configuration

```text
REDIS_ADDR=localhost:6379
HTTP_ADDR=localhost:8080
```

## Local Redis setup

Examples use the local Redis setup from `examples/docker-compose.yml`.

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

Complete it using exact-value CAS:

```shell
curl -X POST 'localhost:8080/statuses/42/complete'
```

Expected result:

```text
HTTP 200
{"swapped":true,"status":"completed",...}
```

The update uses `KeepTTL`, so the existing expiration remains unchanged.

Read the current status:

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

Delete it only when its current value is `processing`:

```shell
curl -X DELETE 'localhost:8080/statuses/7/processing'
```

Expected result:

```text
HTTP 200
{"deleted":true,...}
```

## Stale raw value check

This endpoint seeds `processing`, swaps it to `cancelled`, and then tries to
swap the stale expected value `processing` to `completed`.

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

## Versioned structured values

Orders are stored through:

```go
store, err := xredis.NewVersionedStore[Order](
    client,
    xredis.WithVersionedStorePrefix("xredis:cas:order:"),
)
```

Each order uses one Redis hash:

```text
xredis:cas:order:42
  value     <encoded Order>
  revision  <opaque revision>
```

The encoded previous value is not sent during CAS. Only the expected revision,
the new value, and a new revision are sent.

Keys managed by `VersionedStore[T]` use this dedicated hash representation and
must not be written through regular string-value methods such as `SetStruct`.

## Create a versioned order

```shell
curl -X POST 'localhost:8080/orders/42/seed'
```

Expected result:

```json
{
  "created": true,
  "order": {
    "id": "42",
    "status": "processing",
    "version": 1
  },
  "revision": "<revision>"
}
```

Calling the same endpoint again returns `HTTP 409`, because `Create` does not
overwrite an existing key.

## Read a versioned order

```shell
curl 'localhost:8080/orders/42'
```

The response contains both the decoded order and its current revision.

## Versioned CAS flow

Complete the order using its current revision:

```shell
curl -X POST 'localhost:8080/orders/42/complete'
```

The handler reads the current value and revision, derives the new order state,
and calls `VersionedStore.CompareAndSwap` with `KeepTTL`.

Expected result:

```text
HTTP 200
{"swapped":true,"revision":"<new-revision>",...}
```

Every successful update generates a new opaque revision.

## Versioned CAD flow

Create another order:

```shell
curl -X POST 'localhost:8080/orders/7/seed'
```

Delete it only when the revision read by the handler is still current:

```shell
curl -X DELETE 'localhost:8080/orders/7/current'
```

Expected result:

```text
HTTP 200
{"deleted":true,...}
```

If another client updates the order between the read and delete operations, CAD
returns `false` and the endpoint responds with `HTTP 409`.

## Stale revision check

This endpoint creates an order, updates it from `processing` to `cancelled`,
and then attempts another update using the original stale revision.

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

The first CAS succeeds and returns a new revision. The second CAS uses the
original revision and is rejected.

## Cleanup

Delete the known sample keys:

```shell
curl -X DELETE 'localhost:8080/sample'
```

## Redis Cluster note

Every compare operation in this example executes against one Redis key.

Raw CAS/CAD uses one Redis string key. `VersionedStore[T]` keeps the encoded
value and revision inside one Redis hash. Both layouts work in Redis Cluster
without multi-key hash-slot coordination.

## Stop services

From the repository root:

```shell
docker compose -f examples/docker-compose.yml --profile standalone down --remove-orphans -v
```

Or from this example directory:

```shell
docker compose -f ../docker-compose.yml --profile standalone down --remove-orphans -v
```