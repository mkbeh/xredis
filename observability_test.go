package xredis_test

import (
	"context"
	"time"

	. "github.com/bsm/ginkgo/v2"
	. "github.com/bsm/gomega"
	"github.com/mkbeh/xredis"
	rdb "github.com/redis/go-redis/v9"
)

type testMetrics struct {
	metrics xredis.ClientMetrics
	calls   int
}

func (m *testMetrics) Register() xredis.ClientMetrics {
	m.calls++

	return m.metrics
}

type testCacheMetrics struct {
	requests int
}

func (m *testCacheMetrics) RecordRequest(context.Context, string, string) {
	m.requests++
}

func (*testCacheMetrics) RecordLoaderDuration(context.Context, string, time.Duration) {}

func (*testCacheMetrics) RecordSingleflightShared(context.Context) {}

type testLockMetrics struct {
	operations int
}

func (m *testLockMetrics) RecordOperation(context.Context, string, string, string) {
	m.operations++
}

type testRateLimiterMetrics struct {
	decisions int
}

func (m *testRateLimiterMetrics) RecordDecision(context.Context, string, string, time.Duration) {
	m.decisions++
}

var (
	_ xredis.Metrics            = (*testMetrics)(nil)
	_ xredis.CacheMetrics       = (*testCacheMetrics)(nil)
	_ xredis.LockMetrics        = (*testLockMetrics)(nil)
	_ xredis.RateLimiterMetrics = (*testRateLimiterMetrics)(nil)
)

var _ = Describe("Client observability", func() {
	It("registers metrics once", func() {
		cacheMetrics := &testCacheMetrics{}
		lockMetrics := &testLockMetrics{}
		rateLimiterMetrics := &testRateLimiterMetrics{}

		metrics := &testMetrics{
			metrics: xredis.ClientMetrics{
				Cache:       cacheMetrics,
				Lock:        lockMetrics,
				RateLimiter: rateLimiterMetrics,
			},
		}

		client, err := xredis.NewClient(
			&rdb.Options{
				Addr: redisAddr,
				DB:   testDB,
			},
			xredis.WithMetrics(metrics),
		)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() {
			Expect(client.Close()).To(Succeed())
		})

		Expect(metrics.calls).To(Equal(1))
	})

	It("routes wrapper metrics to their domains", func() {
		cacheMetrics := &testCacheMetrics{}
		lockMetrics := &testLockMetrics{}
		rateLimiterMetrics := &testRateLimiterMetrics{}

		metrics := &testMetrics{
			metrics: xredis.ClientMetrics{
				Cache:       cacheMetrics,
				Lock:        lockMetrics,
				RateLimiter: rateLimiterMetrics,
			},
		}

		client, err := xredis.NewClient(
			&rdb.Options{
				Addr: redisAddr,
				DB:   testDB,
			},
			xredis.WithMetrics(metrics),
		)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() {
			Expect(client.Close()).To(Succeed())
		})

		Expect(client.Raw().FlushDB(ctx).Err()).To(Succeed())

		cache, err := xredis.NewCache[string](
			client,
			xredis.WithCachePrefix("observability:cache:"),
			xredis.WithCacheTTL(time.Minute),
		)
		Expect(err).NotTo(HaveOccurred())

		_, _, err = cache.Get(ctx, "missing")
		Expect(err).NotTo(HaveOccurred())
		Expect(cacheMetrics.requests).To(Equal(1))

		lock, acquired, err := client.TryLock(
			ctx,
			"observability:lock",
			time.Minute,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(acquired).To(BeTrue())
		Expect(lock.Unlock(ctx)).To(Succeed())
		Expect(lockMetrics.operations).To(Equal(2))

		limiter, err := xredis.NewRateLimiter(
			client,
			xredis.WithRateLimiterPrefix("observability:"),
		)
		Expect(err).NotTo(HaveOccurred())

		decision, err := limiter.AllowFixedWindow(
			ctx,
			"rate-limit",
			xredis.RateLimit{
				Limit:  1,
				Window: time.Minute,
			},
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(decision.Allowed).To(BeTrue())
		Expect(rateLimiterMetrics.decisions).To(Equal(1))
	})

	It("supports partially configured metrics", func() {
		metrics := &testMetrics{
			metrics: xredis.ClientMetrics{
				Cache: &testCacheMetrics{},
			},
		}

		client, err := xredis.NewClient(
			&rdb.Options{
				Addr: redisAddr,
				DB:   testDB,
			},
			xredis.WithMetrics(metrics),
		)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() {
			Expect(client.Close()).To(Succeed())
		})

		Expect(client.Raw().FlushDB(ctx).Err()).To(Succeed())

		lock, acquired, err := client.TryLock(
			ctx,
			"observability:partial:lock",
			time.Minute,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(acquired).To(BeTrue())
		Expect(lock.Unlock(ctx)).To(Succeed())

		limiter, err := xredis.NewRateLimiter(
			client,
			xredis.WithRateLimiterPrefix("observability:partial:"),
		)
		Expect(err).NotTo(HaveOccurred())

		_, err = limiter.AllowFixedWindow(
			ctx,
			"rate-limit",
			xredis.RateLimit{
				Limit:  1,
				Window: time.Minute,
			},
		)
		Expect(err).NotTo(HaveOccurred())
	})
})
