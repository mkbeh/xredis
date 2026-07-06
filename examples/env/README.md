# Env Configuration Example

This example shows how to load `xredis.Config` from environment variables using `github.com/caarlos0/env`.

**This example demonstrates:**

* loading Redis configuration from environment variables;
* creating an xredis client from the loaded config;
* checking Redis availability with `PING`;
* storing and reading one string value with TTL.

## Configuration

Configure Redis connection using environment variables:

```text
MODE=standalone
ADDRS=localhost:6379
DB=0
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
Database alias: redis-env
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
go run ./examples/env
```

Expected result:

```text
stored message: key=xredis:env:message value="hello from xredis env example" ttl=1m0s
```

The key is visible in RedisInsight as:

```text
xredis:env:message
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