package xredis_test

import (
	"context"
	"time"

	. "github.com/bsm/ginkgo/v2"
	. "github.com/bsm/gomega"
	"github.com/mkbeh/xredis"
)

type testMetrics struct {
	metrics xredis.ClientMetrics
	calls   int
}

func (m *testMetrics) Register() xredis.ClientMetrics {
	m.calls++

	return m.metrics
}

type testCacheRequest struct {
	operation string
	result    string
}

type testCacheMetrics struct {
	requests []testCacheRequest
}

func (m *testCacheMetrics) RecordRequest(_ context.Context, operation, result string) {
	m.requests = append(m.requests, testCacheRequest{
		operation: operation,
		result:    result,
	})
}

func (*testCacheMetrics) RecordLoaderDuration(context.Context, string, time.Duration) {}

func (*testCacheMetrics) RecordSingleflightShared(context.Context) {}

type testLockOperation struct {
	lockType  string
	operation string
	outcome   string
}

type testLockMetrics struct {
	operations []testLockOperation
}

func (m *testLockMetrics) RecordOperation(_ context.Context, lockType, operation, outcome string) {
	m.operations = append(m.operations, testLockOperation{
		lockType:  lockType,
		operation: operation,
		outcome:   outcome,
	})
}

type testRateLimiterDecision struct {
	algorithm string
	outcome   string
}

type testRateLimiterMetrics struct {
	decisions []testRateLimiterDecision
}

func (m *testRateLimiterMetrics) RecordDecision(_ context.Context, algorithm, outcome string, _ time.Duration) {
	m.decisions = append(m.decisions, testRateLimiterDecision{
		algorithm: algorithm,
		outcome:   outcome,
	})
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

		client := newTestClient(xredis.WithMetrics(metrics))
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

		client := newTestClient(xredis.WithMetrics(metrics))
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
		Expect(cacheMetrics.requests).To(Equal([]testCacheRequest{
			{operation: "get", result: "miss"},
		}))

		lock, acquired, err := client.TryLock(
			ctx,
			"observability:lock",
			time.Minute,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(acquired).To(BeTrue())
		Expect(lock.Unlock(ctx)).To(Succeed())
		Expect(lockMetrics.operations).To(Equal([]testLockOperation{
			{lockType: "lease", operation: "acquire", outcome: "success"},
			{lockType: "lease", operation: "unlock", outcome: "success"},
		}))

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
		Expect(rateLimiterMetrics.decisions).To(Equal([]testRateLimiterDecision{
			{algorithm: "fixed_window", outcome: "allowed"},
		}))
	})

	It("supports partially configured metrics", func() {
		metrics := &testMetrics{
			metrics: xredis.ClientMetrics{
				Cache: &testCacheMetrics{},
			},
		}

		client := newTestClient(xredis.WithMetrics(metrics))
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
