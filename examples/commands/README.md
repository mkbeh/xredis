# Redis Commands REST API

This example shows the main Redis command helpers provided by `xredis`.

**This example demonstrates:**

* Redis health checks with `Ping`
* Raw string values with `Set` and `String`
* Codec-based values with `SetStruct` and `GetStruct`
* Redis hashes with `HSet` and `HGetAll`
* Counters with `Incr` and `Int64`
* Key cleanup with `Delete`

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
Database alias: redis-commands-example
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
go run ./examples/commands
```

The HTTP server starts on:

```text
localhost:8080
```

All examples below use the sample ID `42`.

## Health check

```shell
curl 'localhost:8080/healthz'
```

## Raw string value

Store a raw Redis string with `Set`:

```shell
curl -X PUT 'localhost:8080/messages/42' \
  -H 'Content-Type: application/json' \
  -d '{
    "value": "hello from xredis",
    "ttl_seconds": 300
  }'
```

Read it with `String`:

```shell
curl 'localhost:8080/messages/42'
```

## Codec-based value

Store a Go value with `SetStruct`. The default codec is JSON:

```shell
curl -X PUT 'localhost:8080/profiles/42' \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "Ada Lovelace",
    "email": "ada@example.com",
    "active": true,
    "ttl_seconds": 300
  }'
```

Read it with `GetStruct`:

```shell
curl 'localhost:8080/profiles/42'
```

## Redis hash

Store a Go struct as a Redis hash with `HSet`. Hash field names come from
`redis` struct tags:

```shell
curl -X PUT 'localhost:8080/users/42' \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "Grace Hopper",
    "active": true,
    "ttl_seconds": 300
  }'
```

Read it with `HGetAll`:

```shell
curl 'localhost:8080/users/42'
```

## Counter

Increment a counter with `Incr` and read it with `Int64`:

```shell
curl -X POST 'localhost:8080/counters/42/increment'
```

Run the request multiple times to increment the value.

## Cleanup

Delete all keys created for sample ID `42`:

```shell
curl -X DELETE 'localhost:8080/sample/42'
```

## Additional helpers

Other command helpers include conditional writes, atomic read-and-delete or
read-and-expire operations, individual hash field operations, scalar getters,
and struct-specific variants.

## Stop services

From the repository root:

```shell
docker compose -f examples/docker-compose.yml --profile standalone down --remove-orphans -v
```

Or from this example directory:

```shell
docker compose -f ../docker-compose.yml --profile standalone down --remove-orphans -v
```