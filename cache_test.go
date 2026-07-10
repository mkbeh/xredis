package xredis_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	. "github.com/bsm/ginkgo/v2"
	. "github.com/bsm/gomega"
	"github.com/mkbeh/xredis"
)

type cacheUser struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

type cacheUserResult struct {
	value cacheUser
	err   error
}

var _ = Describe("Cache", func() {
	var client *xredis.Client

	BeforeEach(func() {
		client = newTestClient()
		Expect(client.Raw().FlushDB(ctx).Err()).To(Succeed())
	})

	AfterEach(func() {
		Expect(client.Close()).To(Succeed())
	})

	Describe("singleflight", func() {
		It("shares one loader execution between concurrent cache misses", func() {
			cache, err := xredis.NewCache[cacheUser](
				client,
				xredis.WithCachePrefix("cache:singleflight:"),
				xredis.WithCacheTTL(time.Minute),
			)
			Expect(err).NotTo(HaveOccurred())

			expected := cacheUser{
				ID:     "42",
				Name:   "Ada",
				Active: true,
			}

			var loads atomic.Int64

			const callers = 20

			start := make(chan struct{})
			results := make(chan cacheUserResult, callers)

			var wg sync.WaitGroup
			wg.Add(callers)

			for range callers {
				go func() {
					defer wg.Done()

					<-start

					value, loadErr := cache.GetOrLoad(
						context.Background(),
						"42",
						func(context.Context) (cacheUser, error) {
							loads.Add(1)
							time.Sleep(100 * time.Millisecond)

							return expected, nil
						},
					)

					results <- cacheUserResult{
						value: value,
						err:   loadErr,
					}
				}()
			}

			close(start)
			wg.Wait()
			close(results)

			for result := range results {
				Expect(result.err).NotTo(HaveOccurred())
				Expect(result.value).To(Equal(expected))
			}

			Expect(loads.Load()).To(Equal(int64(1)))
		})
	})

	Describe("interface values", func() {
		It("supports a nil value loaded into Cache[any]", func() {
			cache, err := xredis.NewCache[any](
				client,
				xredis.WithCachePrefix("cache:any:"),
				xredis.WithCacheTTL(time.Minute),
			)
			Expect(err).NotTo(HaveOccurred())

			value, err := cache.GetOrLoad(
				ctx,
				"value",
				func(context.Context) (any, error) {
					return nil, nil
				},
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(value).To(BeNil())

			value, ok, err := cache.Get(ctx, "value")
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(value).To(BeNil())
		})
	})

	Describe("not-found handling", func() {
		It("normalizes a custom not-found error", func() {
			errUserNotFound := errors.New("user not found")

			cache, err := xredis.NewCache[cacheUser](
				client,
				xredis.WithCachePrefix("cache:custom-not-found:"),
				xredis.WithCacheTTL(time.Minute),
				xredis.WithCacheNegativeTTL(time.Minute),
				xredis.WithCacheNotFound(func(err error) bool {
					return errors.Is(err, errUserNotFound)
				}),
			)
			Expect(err).NotTo(HaveOccurred())

			_, err = cache.GetOrLoad(
				ctx,
				"404",
				func(context.Context) (cacheUser, error) {
					return cacheUser{}, errUserNotFound
				},
			)

			Expect(errors.Is(err, xredis.ErrKeyNotFound)).To(BeTrue())
			Expect(errors.Is(err, errUserNotFound)).To(BeTrue())
		})

		It("caches not-found results and skips repeated loader calls", func() {
			cache, err := xredis.NewCache[cacheUser](
				client,
				xredis.WithCachePrefix("cache:negative:"),
				xredis.WithCacheTTL(time.Minute),
				xredis.WithCacheNegativeTTL(time.Minute),
			)
			Expect(err).NotTo(HaveOccurred())

			var loads atomic.Int64

			loader := func(context.Context) (cacheUser, error) {
				loads.Add(1)

				return cacheUser{}, xredis.ErrKeyNotFound
			}

			_, err = cache.GetOrLoad(ctx, "404", loader)
			Expect(errors.Is(err, xredis.ErrKeyNotFound)).To(BeTrue())

			_, err = cache.GetOrLoad(ctx, "404", loader)
			Expect(errors.Is(err, xredis.ErrKeyNotFound)).To(BeTrue())

			Expect(loads.Load()).To(Equal(int64(1)))

			value, ok, err := cache.Get(ctx, "404")
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeFalse())
			Expect(value).To(Equal(cacheUser{}))
		})
	})

	Describe("entry validation", func() {
		It("rejects a corrupted negative cache entry", func() {
			const prefix = "cache:corrupted:"

			cache, err := xredis.NewCache[string](
				client,
				xredis.WithCachePrefix(prefix),
				xredis.WithCacheTTL(time.Minute),
			)
			Expect(err).NotTo(HaveOccurred())

			Expect(client.Raw().Set(
				ctx,
				prefix+"value",
				[]byte{0, 1},
				time.Minute,
			).Err()).To(Succeed())

			value, ok, err := cache.Get(ctx, "value")
			Expect(errors.Is(err, xredis.ErrInvalidCacheEntry)).To(BeTrue())
			Expect(ok).To(BeFalse())
			Expect(value).To(BeEmpty())
		})
	})

	Describe("compare operations", func() {
		It("swaps and deletes cache values using the cache format", func() {
			cache, err := xredis.NewCache[string](
				client,
				xredis.WithCachePrefix("cache:cas:"),
				xredis.WithCacheTTL(time.Minute),
			)
			Expect(err).NotTo(HaveOccurred())

			Expect(cache.Set(ctx, "42", "processing")).To(Succeed())

			swapped, err := cache.CompareAndSwap(
				ctx,
				"42",
				"pending",
				"completed",
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(swapped).To(BeFalse())

			swapped, err = cache.CompareAndSwap(
				ctx,
				"42",
				"processing",
				"completed",
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(swapped).To(BeTrue())

			value, ok, err := cache.Get(ctx, "42")
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(value).To(Equal("completed"))

			deleted, err := cache.CompareAndDelete(ctx, "42", "processing")
			Expect(err).NotTo(HaveOccurred())
			Expect(deleted).To(BeFalse())

			deleted, err = cache.CompareAndDelete(ctx, "42", "completed")
			Expect(err).NotTo(HaveOccurred())
			Expect(deleted).To(BeTrue())

			value, ok, err = cache.Get(ctx, "42")
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeFalse())
			Expect(value).To(BeEmpty())
		})

		It("does not match a negative entry as a regular cache value", func() {
			cache, err := xredis.NewCache[string](
				client,
				xredis.WithCachePrefix("cache:negative-cas:"),
				xredis.WithCacheTTL(time.Minute),
				xredis.WithCacheNegativeTTL(time.Minute),
			)
			Expect(err).NotTo(HaveOccurred())

			_, err = cache.GetOrLoad(
				ctx,
				"404",
				func(context.Context) (string, error) {
					return "", xredis.ErrKeyNotFound
				},
			)
			Expect(errors.Is(err, xredis.ErrKeyNotFound)).To(BeTrue())

			swapped, err := cache.CompareAndSwap(
				ctx,
				"404",
				"",
				"value",
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(swapped).To(BeFalse())

			deleted, err := cache.CompareAndDelete(ctx, "404", "")
			Expect(err).NotTo(HaveOccurred())
			Expect(deleted).To(BeFalse())
		})
	})
})
