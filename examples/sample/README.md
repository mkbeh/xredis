# Sample REST API

This example shows how to use `xredis` in a simple REST API service.

**This example demonstrates:**

* standalone Redis client configuration;
* Redis health check with `PING`;
* string values with TTL;
* hash values from Go structs;
* direct `go-redis` access through `Raw`.

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
docker compose -f ./docker-compose.yml --profile standalone up -d
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
Database alias: redis-sample
Host: redis-standalone
Port: 6379
Username: default
Password: empty
Database index: 0
```

## Run

From this directory:

```shell
go run main.go
```

Or from the repository root:

```shell
go run ./examples/sample
```

The HTTP server starts on:

```text
localhost:8080
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

## Set message

Stores a string value with TTL.

```shell
curl -X PUT 'localhost:8080/message' \
  -H 'Content-Type: application/json' \
  -d '{
    "value": "hello from xredis",
    "ttl_seconds": 60
  }'
```

Expected result:

```text
HTTP 200
message is visible in RedisInsight by key xredis:sample:message
```

## Get message

Reads the stored string value.

```shell
curl 'localhost:8080/message'
```

Expected result:

```text
HTTP 200
{"key":"xredis:sample:message","ttl":"...","value":"hello from xredis"}
```

## Increment counter

Increments a counter using the raw `go-redis` client.

```shell
curl -X POST 'localhost:8080/counter/increment'
```

Expected result:

```text
HTTP 200
counter value is incremented
```

The key is visible in RedisInsight as:

```text
xredis:sample:counter
```

## Set user

Stores a user as a Redis hash.

```shell
curl -X PUT 'localhost:8080/users/42' \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "Ada Lovelace",
    "age": 36,
    "active": true,
    "ttl_seconds": 300
  }'
```

Expected result:

```text
HTTP 200
user hash is visible in RedisInsight by key xredis:sample:user:42
```

## Get user

Reads the user hash back into a Go struct.

```shell
curl 'localhost:8080/users/42'
```

Expected result:

```text
HTTP 200
{"key":"xredis:sample:user:42","ttl":"...","user":{"id":"42","name":"Ada Lovelace","age":36,"active":true}}
```

## Cleanup

Deletes all sample keys by prefix.

```shell
curl -X DELETE 'localhost:8080/sample'
```

Expected result:

```text
HTTP 200
all keys with xredis:sample:* prefix are deleted
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