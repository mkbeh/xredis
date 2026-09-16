package xredis_test

import (
	"context"
	"os"
	"testing"
	"time"

	. "github.com/bsm/ginkgo/v2"
	. "github.com/bsm/gomega"
	"github.com/mkbeh/xredis"
	rdb "github.com/redis/go-redis/v9"
)

const (
	defaultRedisAddr = "localhost:6379"
	testDB           = 15
)

var (
	ctx       = context.Background()
	redisAddr = defaultRedisAddr
)

func TestGinkgoSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "xredis")
}

var _ = BeforeSuite(func() {
	if addr := os.Getenv("REDIS_ADDR"); addr != "" {
		redisAddr = addr
	}

	client := newTestClient()
	defer func() {
		Expect(client.Close()).To(Succeed())
	}()

	Expect(client.Ping(ctx)).To(Succeed())
})

func newTestClient(opts ...xredis.Option) *xredis.Client {
	GinkgoHelper()

	client, err := xredis.NewClient(
		&rdb.Options{
			Addr:         redisAddr,
			DB:           testDB,
			ClientName:   "xredis-test",
			DialTimeout:  5 * time.Second,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 5 * time.Second,
		},
		opts...,
	)
	Expect(err).NotTo(HaveOccurred())

	return client
}
