package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/mkbeh/xredis"
)

const (
	exampleClient = "env-example-client"
	exampleKey    = "xredis:env:message"
	exampleValue  = "hello from xredis env example"
	exampleTTL    = time.Minute
)

func main() {
	ctx := context.Background()

	cfg, err := env.ParseAsWithOptions[xredis.ClientConfig](env.Options{
		UseFieldNameByDefault: true,
	})
	if err != nil {
		log.Fatalln(err)
	}

	client, err := xredis.NewClient(
		xredis.WithClientConfig(&cfg),
		xredis.WithClientID(exampleClient),
	)
	if err != nil {
		log.Fatalln(err)
	}
	defer func() {
		if closeErr := client.Close(); closeErr != nil {
			log.Println("unable to close Redis client:", closeErr)
		}
	}()

	if err = client.Ping(ctx); err != nil {
		log.Fatalln(err)
	}

	if err = client.Set(ctx, exampleKey, exampleValue, exampleTTL); err != nil {
		log.Fatalln(err)
	}

	value, ok, err := client.String(ctx, exampleKey)
	if err != nil {
		log.Fatalln(err)
	}
	if !ok {
		log.Fatalf("key %q was not found", exampleKey)
	}

	fmt.Printf(
		"stored message: key=%s value=%q ttl=%s\n",
		exampleKey,
		value,
		exampleTTL,
	)
}
