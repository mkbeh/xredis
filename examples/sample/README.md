# Description

This is a sample REST API service implemented using go-redis as the connector to a Redis data store.

# Usage

Setup a Redis database.

```shell
docker-compose up --build -d
```

Configure the database connection with environment variables:

```text
REDIS_ADDRS=localhost:6379
```

Run main.go:

```
go run main.go
```

## Create tasks

```shell
curl '127.0.0.1:8080/create'
```

## Get tasks

```shell
curl '127.0.0.1:8080/get'
```

## Metrics

```shell
curl 'http://localhost:8080/metrics'
```