## Example: Redis Pipeline REST API

This example demonstrates how to use xredis pipeline helpers for grouped Redis operations.

### Key Concepts

* **Raw Batch Writes** — Store multiple raw Redis values with `SetItems`.
* **Structured Batch Writes** — Encode and store multiple structured values with `SetStructItems`.
* **Key Deletion** — Delete known keys with `DeleteKeys`.
* **Asynchronous Unlinking** — Remove known keys with `UnlinkKeys` while reclaiming memory asynchronously.
* **Cluster-Safe Pipelines** — Use single-key pipeline operations that work with standalone Redis, Redis Cluster, and Ring clients.

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
Database alias: redis-pipeline-example
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
go run ./examples/pipeline
```

The HTTP server starts on:

```text
localhost:8080
```

## Health check

```shell
curl 'localhost:8080/healthz'
```

## Seed sample keys

Creates raw message and counter values with `SetItems`, then creates profile,
settings, delete, and unlink values with `SetStructItems`.

```shell
curl -X POST 'localhost:8080/sample/42'
```

Seed more keys:

```shell
curl -X POST 'localhost:8080/sample/7'
curl -X POST 'localhost:8080/sample/100'
```

You can inspect created keys in RedisInsight with the `xredis:pipeline:` prefix.

`SetItems` passes raw values directly to Redis. In this example it stores a
string and an integer.

`SetStructItems` encodes values with the configured `xredis` codec. With the
default JSON codec, the sample structs are stored as JSON objects.

## Delete keys

`DeleteKeys` deletes known delete sample keys with `DEL`.

```shell
curl -X DELETE 'localhost:8080/pipeline/delete/42'
```

Check RedisInsight: keys matching `xredis:pipeline:delete:42:*` should be removed.

## Unlink keys

`UnlinkKeys` unlinks known unlink sample keys with `UNLINK`.

`UNLINK` removes keys from the keyspace and reclaims memory asynchronously, which is useful when values may be large.

```shell
curl -X DELETE 'localhost:8080/pipeline/unlink/42'
```

Check RedisInsight: keys matching `xredis:pipeline:unlink:42:*` should be removed.

## Cleanup

Deletes all known sample keys for the IDs used by this example with `DeleteKeys`.

```shell
curl -X DELETE 'localhost:8080/sample'
```

## Redis Cluster note

`SetItems`, `SetStructItems`, `DeleteKeys`, and `UnlinkKeys` support standalone
Redis, Redis Cluster, and Ring clients.

`SetItems` and `SetStructItems` write values as independent single-key `SET`
commands.

For standalone Redis, `DeleteKeys` and `UnlinkKeys` use one multi-key command.
For Redis Cluster and Ring clients, they use pipelined single-key commands to
avoid multi-key hash-slot constraints.

Pipeline helpers reduce network round trips but do not provide all-or-nothing
semantics. If execution fails, some commands may already have succeeded.

## Stop services

From the repository root:

```shell
docker compose -f examples/docker-compose.yml --profile standalone down --remove-orphans -v
```

Or from this example directory:

```shell
docker compose -f ../docker-compose.yml --profile standalone down --remove-orphans -v
```