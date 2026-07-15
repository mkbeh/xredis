package xredis_test

import (
	"time"

	. "github.com/bsm/ginkgo/v2"
	. "github.com/bsm/gomega"
	"github.com/mkbeh/xredis"
)

const rateLimitTestWindow = 200 * time.Millisecond

var _ = Describe("Rate limiter", func() {
	var (
		client  *xredis.Client
		limiter *xredis.RateLimiter
	)

	BeforeEach(func() {
		client = newTestClient()
		Expect(client.Raw().FlushDB(ctx).Err()).To(Succeed())

		var err error
		limiter, err = client.RateLimiter(
			xredis.WithRateLimiterPrefix("rate-limit:"),
		)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(client.Close()).To(Succeed())
	})

	Describe("fixed window", func() {
		It("allows requests up to the limit and rejects the next request", func() {
			limit := xredis.RateLimit{
				Limit:  2,
				Window: rateLimitTestWindow,
			}

			first, err := limiter.AllowFixedWindow(ctx, "fixed:user:42", limit)
			Expect(err).NotTo(HaveOccurred())
			Expect(first.Allowed).To(BeTrue())
			Expect(first.Limit).To(Equal(int64(2)))
			Expect(first.Remaining).To(Equal(int64(1)))
			Expect(first.RetryAfter).To(BeZero())
			Expect(first.ResetAfter).To(BeNumerically(">", 0))
			Expect(first.ResetAfter).To(BeNumerically("<=", rateLimitTestWindow))

			second, err := limiter.AllowFixedWindow(ctx, "fixed:user:42", limit)
			Expect(err).NotTo(HaveOccurred())
			Expect(second.Allowed).To(BeTrue())
			Expect(second.Limit).To(Equal(int64(2)))
			Expect(second.Remaining).To(BeZero())
			Expect(second.RetryAfter).To(BeZero())
			Expect(second.ResetAfter).To(BeNumerically(">", 0))

			rejected, err := limiter.AllowFixedWindow(ctx, "fixed:user:42", limit)
			Expect(err).NotTo(HaveOccurred())
			Expect(rejected.Allowed).To(BeFalse())
			Expect(rejected.Limit).To(Equal(int64(2)))
			Expect(rejected.Remaining).To(BeZero())
			Expect(rejected.RetryAfter).To(BeNumerically(">", 0))
			Expect(rejected.ResetAfter).To(BeNumerically(">", 0))
			Expect(rejected.RetryAfter).To(Equal(rejected.ResetAfter))
		})

		It("starts a new window after the current window expires", func() {
			limit := xredis.RateLimit{
				Limit:  1,
				Window: rateLimitTestWindow,
			}

			decision, err := limiter.Allow(ctx, "fixed:reset:42", limit)
			Expect(err).NotTo(HaveOccurred())
			Expect(decision.Allowed).To(BeTrue())

			decision, err = limiter.Allow(ctx, "fixed:reset:42", limit)
			Expect(err).NotTo(HaveOccurred())
			Expect(decision.Allowed).To(BeFalse())

			Eventually(func() bool {
				next, allowErr := limiter.Allow(ctx, "fixed:reset:42", limit)

				return allowErr == nil && next.Allowed
			}, 2*time.Second, 20*time.Millisecond).Should(BeTrue())
		})
	})

	Describe("sliding window", func() {
		It("allows requests up to the limit and rejects the next request", func() {
			limit := xredis.RateLimit{
				Limit:  2,
				Window: rateLimitTestWindow,
			}

			first, err := limiter.AllowSlidingWindow(ctx, "sliding:user:42", limit)
			Expect(err).NotTo(HaveOccurred())
			Expect(first.Allowed).To(BeTrue())
			Expect(first.Limit).To(Equal(int64(2)))
			Expect(first.Remaining).To(Equal(int64(1)))
			Expect(first.RetryAfter).To(BeZero())
			Expect(first.ResetAfter).To(BeNumerically(">", 0))
			Expect(first.ResetAfter).To(BeNumerically("<=", rateLimitTestWindow))

			second, err := limiter.AllowSlidingWindow(ctx, "sliding:user:42", limit)
			Expect(err).NotTo(HaveOccurred())
			Expect(second.Allowed).To(BeTrue())
			Expect(second.Limit).To(Equal(int64(2)))
			Expect(second.Remaining).To(BeZero())
			Expect(second.RetryAfter).To(BeZero())
			Expect(second.ResetAfter).To(BeNumerically(">", 0))

			rejected, err := limiter.AllowSlidingWindow(ctx, "sliding:user:42", limit)
			Expect(err).NotTo(HaveOccurred())
			Expect(rejected.Allowed).To(BeFalse())
			Expect(rejected.Limit).To(Equal(int64(2)))
			Expect(rejected.Remaining).To(BeZero())
			Expect(rejected.RetryAfter).To(BeNumerically(">", 0))
			Expect(rejected.ResetAfter).To(BeNumerically(">", 0))
			Expect(rejected.RetryAfter).To(Equal(rejected.ResetAfter))
		})

		It("allows a request after the oldest entry leaves the window", func() {
			limit := xredis.RateLimit{
				Limit:  1,
				Window: rateLimitTestWindow,
			}

			decision, err := limiter.AllowSlidingWindow(
				ctx,
				"sliding:reset:42",
				limit,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(decision.Allowed).To(BeTrue())

			decision, err = limiter.AllowSlidingWindow(
				ctx,
				"sliding:reset:42",
				limit,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(decision.Allowed).To(BeFalse())

			Eventually(func() bool {
				next, allowErr := limiter.AllowSlidingWindow(
					ctx,
					"sliding:reset:42",
					limit,
				)

				return allowErr == nil && next.Allowed
			}, 2*time.Second, 20*time.Millisecond).Should(BeTrue())
		})
	})

	Describe("token bucket", func() {
		It("allows a burst up to capacity and rejects the next request", func() {
			limit := xredis.TokenBucketRateLimit{
				Limit:  2,
				Window: 10 * time.Second,
				Burst:  3,
			}

			first, err := limiter.AllowTokenBucket(ctx, "bucket:user:42", limit)
			Expect(err).NotTo(HaveOccurred())
			Expect(first.Allowed).To(BeTrue())
			Expect(first.Limit).To(Equal(int64(3)))
			Expect(first.Remaining).To(Equal(int64(2)))
			Expect(first.RetryAfter).To(BeZero())
			Expect(first.ResetAfter).To(BeNumerically(">", 0))

			second, err := limiter.AllowTokenBucket(ctx, "bucket:user:42", limit)
			Expect(err).NotTo(HaveOccurred())
			Expect(second.Allowed).To(BeTrue())
			Expect(second.Limit).To(Equal(int64(3)))
			Expect(second.Remaining).To(Equal(int64(1)))
			Expect(second.RetryAfter).To(BeZero())
			Expect(second.ResetAfter).To(BeNumerically(">", 0))

			third, err := limiter.AllowTokenBucket(ctx, "bucket:user:42", limit)
			Expect(err).NotTo(HaveOccurred())
			Expect(third.Allowed).To(BeTrue())
			Expect(third.Limit).To(Equal(int64(3)))
			Expect(third.Remaining).To(BeZero())
			Expect(third.RetryAfter).To(BeZero())
			Expect(third.ResetAfter).To(BeNumerically(">", 0))

			rejected, err := limiter.AllowTokenBucket(ctx, "bucket:user:42", limit)
			Expect(err).NotTo(HaveOccurred())
			Expect(rejected.Allowed).To(BeFalse())
			Expect(rejected.Limit).To(Equal(int64(3)))
			Expect(rejected.Remaining).To(BeZero())
			Expect(rejected.RetryAfter).To(BeNumerically(">", 0))
			Expect(rejected.ResetAfter).To(BeNumerically(">", 0))
		})

		It("allows another request after a token is refilled", func() {
			limit := xredis.TokenBucketRateLimit{
				Limit:  1,
				Window: rateLimitTestWindow,
				Burst:  1,
			}

			decision, err := limiter.AllowTokenBucket(
				ctx,
				"bucket:refill:42",
				limit,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(decision.Allowed).To(BeTrue())
			Expect(decision.Remaining).To(BeZero())

			decision, err = limiter.AllowTokenBucket(
				ctx,
				"bucket:refill:42",
				limit,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(decision.Allowed).To(BeFalse())
			Expect(decision.RetryAfter).To(BeNumerically(">", 0))

			Eventually(func() bool {
				next, allowErr := limiter.AllowTokenBucket(
					ctx,
					"bucket:refill:42",
					limit,
				)

				return allowErr == nil && next.Allowed
			}, 2*time.Second, 20*time.Millisecond).Should(BeTrue())
		})
	})
})
