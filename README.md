# Redis Library

This library provides an API for working with Redis, using [go-redis](github.com/redis/go-redis) and
integration with OpenTelemetry for tracing and metrics.

## Features

- Client and Cluster supporting
- Observability

## Getting started

Here's a basic overview of using (more examples can be
found [here](https://github.com/mkbeh/redis/tree/main/examples/sample)):

```go
package main

import (
	"context"
	"fmt"
	"github.com/mkbeh/redis"
)

var ctx = context.Background()

func main() {
	cfg := &redis.Config{
		Addrs: "localhost:6379",
	}

	client, err := redis.NewClient(
		redis.WithConfig(cfg),
		redis.WithClientID("test-client"),
	)
	if err != nil {
		panic(err)
	}
	defer client.Close()

	if err := client.Set(ctx, "value_1", "First value", 0); err != nil {
		panic(err)
	}

	var value string
	if err := client.Get(ctx, "value_1", &value); err != nil {
		panic(err)
	}

	fmt.Println("key1", value)
}

```

## Configuration

| ENV                           | Description                                                                                                                                                                                                                                                                 |
|-------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| REDIS_NETWORK                 | The network type, either tcp or unix. Default is tcp.                                                                                                                                                                                                                       |
| REDIS_ADDRS                   | A seed list of host:port addresses of cluster nodes.                                                                                                                                                                                                                        |
| REDIS_PROTOCOL                | Protocol 2 or 3. Use the version to negotiate RESP version with redis-server. Default is 3.                                                                                                                                                                                 |
| REDIS_USERNAME                | Use the specified Username to authenticate the current connection with one of the connections defined in the ACL list when connecting to a Redis 6.0 instance, or greater, that is using the Redis ACL system.                                                              |
| REDIS_PASSWORD                | Optional password. Must match the password specified in the requirepass server configuration option (if connecting to a Redis 5.0 instance, or lower), or the User Password when connecting to a Redis 6.0 instance, or greater, that is using the Redis ACL system.        |
| REDIS_DB                      | Database to be selected after connecting to the server.                                                                                                                                                                                                                     |
| REDIS_MAX_REDIRECTS           | The maximum number of retries before giving up. Command is retried on network errors and MOVED/ASK redirects. Default is 3 retries.                                                                                                                                         |
| REDIS_READONLY                | Enables read-only commands on slave nodes.                                                                                                                                                                                                                                  |
| REDIS_ROUTE_BY_LATENCY        | Allows routing read-only commands to the closest master or slave node. It automatically enables ReadOnly.                                                                                                                                                                   |
| REDIS_ROUTE_RANDOMLY          | Allows routing read-only commands to the random master or slave node. It automatically enables ReadOnly.                                                                                                                                                                    |
| REDIS_MAX_RETRIES             | Maximum number of retries before giving up. Default is 3 retries; -1 (not 0) disables retries.                                                                                                                                                                              |
| REDIS_MIN_RETRY_BACKOFF       | Minimum backoff between each retry. Default is 8 milliseconds; -1 disables backoff.                                                                                                                                                                                         |
| REDIS_MAX_RETRY_BACKOFF       | Maximum backoff between each retry. Default is 512 milliseconds; -1 disables backoff.                                                                                                                                                                                       |
| REDIS_DIAL_TIMEOUT            | Dial timeout for establishing new connections. Default is 5 seconds.                                                                                                                                                                                                        |
| REDIS_READ_TIMEOUT            | Timeout for socket reads. If reached, commands will fail with a timeout instead of blocking. Supported values: `0` - default timeout (3 seconds), `-1` - no timeout (block indefinitely), `-2` - disables SetReadDeadline calls completely.                                 |
| REDIS_WRITE_TIMEOUT           | Timeout for socket writes. If reached, commands will fail with a timeout instead of blocking. Supported values: `0` - default timeout (3 seconds), `-1` - no timeout (block indefinitely), `-2` - disables SetReadDeadline calls completely.                                |
| REDIS_CONTEXT_TIMEOUT_ENABLED | ContextTimeoutEnabled controls whether the client respects context timeouts and deadlines. See https://redis.uptrace.dev/guide/go-redis-debugging.html#timeouts                                                                                                             |
| REDIS_POOL_FIFO               | Type of connection pool. true for FIFO pool, false for LIFO pool. Note that FIFO has slightly higher overhead compared to LIFO, but it helps closing idle connections faster reducing the pool size.                                                                        |
| REDIS_POOL_SIZE               | Base number of socket connections. Default is 10 connections per every available CPU as reported by runtime.GOMAXPROCS. If there is not enough connections in the pool, new connections will be allocated in excess of PoolSize, you can limit it through MaxActiveConns    |
| REDIS_POOL_TIMEOUT            | Amount of time client waits for connection if all connections are busy before returning an error. Default is ReadTimeout + 1 second.                                                                                                                                        |
| REDIS_MIN_IDLE_CONNS          | Minimum number of idle connections which is useful when establishing new connection is slow. Default is 0. the idle connections are not closed by default.                                                                                                                  |
| REDIS_MAX_IDLE_CONNS          | Maximum number of idle connections. Default is 0. the idle connections are not closed by default.                                                                                                                                                                           |
| REDIS_MAX_ACTIVE_CONNS        | Maximum number of connections allocated by the pool at a given time. When zero, there is no limit on the number of connections in the pool.                                                                                                                                 |
| REDIS_CONN_MAX_IDLE_TIME      | Maximum amount of time a connection may be idle. Should be less than server's timeout. Expired connections may be closed lazily before reuse. If d <= 0, connections are not closed due to a connection's idle time. Default is 30 minutes. -1 disables idle timeout check. |
| REDIS_CONN_MAX_LIFETIME       | Maximum amount of time a connection may be reused. Expired connections may be closed lazily before reuse. If <= 0, connections are not closed due to a connection's age.                                                                                                    |
| REDIS_DISABLE_INDENTITY       | Disable set-lib on connect. Default is false.                                                                                                                                                                                                                               |
| REDIS_UNSTABLE_RESP3          | Enable Unstable mode for Redis Search module with RESP3.                                                                                                                                                                                                                    |
