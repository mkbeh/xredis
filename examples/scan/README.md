# Redis SCAN REST API

This example shows how to use Redis SCAN helpers with `xredis`.

**This example demonstrates:**

* Cursor-based page scan with `Scan`
* Full scan with `ScanAll`
* Batch iteration with `ScanEachBatch`
* Type filtering with `ScanOptions.Type`
* Pattern deletion with `ScanDelete`
* Pattern unlinking with `ScanUnlink`
* Cluster and Ring topology-wide scans

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
Database alias: redis-scan-example
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
go run ./examples/scan
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

Creates string, hash, stream, delete, and unlink sample keys.

```shell
curl -X POST 'localhost:8080/sample/42'
```

Seed more keys:

```shell
curl -X POST 'localhost:8080/sample/7'
curl -X POST 'localhost:8080/sample/100'
```

## Page scan

`Scan` reads one cursor page.

```shell
curl 'localhost:8080/scan/page?match=xredis:scan:*&count=5'
```

The response contains `next_cursor`.

Continue from the returned numeric `next_cursor` value:

```shell
curl 'localhost:8080/scan/page?match=xredis:scan:*&count=5&cursor=12'
```

Replace 12 with the next_cursor value from the previous response.

The scan is complete when `next_cursor` is `0`.

`Scan` represents one cursor sequence. For topology-wide Cluster or Ring scans,
use `ScanAll`, `ScanEach`, or `ScanEachBatch`.

## Scan all

`ScanAll` scans all matching keys and returns them as a slice.

```shell
curl 'localhost:8080/scan/all?match=xredis:scan:*&count=100'
```

For large keyspaces, prefer `ScanEach` or `ScanEachBatch` because `ScanAll` stores all keys in memory.

## Scan batches

`ScanEachBatch` calls the handler once per SCAN page.

For Redis Cluster and Ring clients, callbacks from different nodes or shards may
run concurrently. Shared state in the callback must be synchronized.

```shell
curl 'localhost:8080/scan/batches?match=xredis:scan:*&count=5'
```

## Type-filtered scan

Scan only string keys:

```shell
curl 'localhost:8080/scan/all?match=xredis:scan:*&type=string&count=100'
```

Scan only hash keys:

```shell
curl 'localhost:8080/scan/all?match=xredis:scan:*&type=hash&count=100'
```

Scan only stream keys:

```shell
curl 'localhost:8080/scan/all?match=xredis:scan:*&type=stream&count=100'
```

## Delete by pattern

`ScanDelete` scans keys and deletes them using pipelined single-key `DEL` commands.

```shell
curl -X DELETE 'localhost:8080/scan/delete/42'
```

Check that delete sample keys are gone:

```shell
curl 'localhost:8080/scan/all?match=xredis:scan:delete:42:*&count=100'
```

## Unlink by pattern

`ScanUnlink` scans keys and unlinks them using pipelined single-key `UNLINK` commands.

`UNLINK` removes keys from the keyspace and reclaims memory asynchronously,
which is preferable for large values.

```shell
curl -X DELETE 'localhost:8080/scan/unlink/42'
```

Check that unlink sample keys are gone:

```shell
curl 'localhost:8080/scan/all?match=xredis:scan:unlink:42:*&count=100'
```

## Cleanup

Deletes all sample keys.

```shell
curl -X DELETE 'localhost:8080/sample'
```

## Redis Cluster note

`ScanEach`, `ScanEachBatch`, `ScanAll`, `ScanDelete`, and `ScanUnlink` are
topology-aware.

For Redis Cluster clients, they scan every master node. For Redis Ring clients,
they scan every live shard. Each node or shard starts from cursor `0`, because
SCAN cursors are node-local.

Callbacks passed to `ScanEach` and `ScanEachBatch` may run concurrently across
nodes or shards. Synchronize access to shared state.

`ScanDelete` and `ScanUnlink` use pipelined single-key commands to avoid
multi-key hash-slot constraints.

## Stop services

From the repository root:

```shell
docker compose -f examples/docker-compose.yml --profile standalone down --remove-orphans -v
```

Or from this example directory:

```shell
docker compose -f ../docker-compose.yml --profile standalone down --remove-orphans -v
```