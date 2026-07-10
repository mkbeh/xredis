package xredis_test

import (
	"context"
	"os"
	"testing"
	"time"

	. "github.com/bsm/ginkgo/v2"
	. "github.com/bsm/gomega"
	"github.com/mkbeh/xredis"
)

const (
	defaultRedisAddr = "localhost:6379"
	testDB           = 15
)

var (
	ctx       = context.TODO()
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

func newTestClient() *xredis.Client {
	client, err := xredis.NewClient(
		xredis.WithClientConfig(&xredis.ClientConfig{
			Addr:         redisAddr,
			DB:           testDB,
			DialTimeout:  5 * time.Second,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 5 * time.Second,
		}),
		xredis.WithClientID("xredis-test"),
	)
	Expect(err).NotTo(HaveOccurred())

	return client
}
