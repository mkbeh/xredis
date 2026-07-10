# Environment Configuration

This example shows how to load `xredis.ClientConfig` from environment variables using `github.com/caarlos0/env`.

**This example demonstrates:**

* Loading Redis configuration from environment variables
* Creating an `xredis` client from the parsed configuration
* Redis health checks with `Ping`
* Storing and reading a raw string value with `Set` and `String`

## Configuration

The example uses `UseFieldNameByDefault`, so environment variable names are derived directly from `ClientConfig` field names without configuration tags.

```shell
export MODE=standalone
export ADDRS=localhost:6379
export DB=0
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
Database alias: redis-env-example
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