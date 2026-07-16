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

type cacheUserPointer *cacheUser

type cacheUserID int64

type cacheUserResult struct {
	value cacheUser
	err   error
}

type cacheCodecSpy struct {
	marshalCalls   atomic.Int64
	unmarshalCalls atomic.Int64
}

func (c *cacheCodecSpy) Marshal(value any) ([]byte, error) {
	c.marshalCalls.Add(1)

	return (xredis.JSONCodec{}).Marshal(value)
}

func (c *cacheCodecSpy) Unmarshal(data []byte, dst any) error {
	c.unmarshalCalls.Add(1)

	return (xredis.JSONCodec{}).Unmarshal(data, dst)
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

	Describe("type validation", func() {
		It("rejects interface cache types", func() {
			cache, err := xredis.NewCache[any](
				client,
				xredis.WithCachePrefix("cache:any:"),
				xredis.WithCacheTTL(time.Minute),
			)

			Expect(cache).To(BeNil())
			Expect(errors.Is(err, xredis.ErrUnsupportedType)).To(BeTrue())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("interface value type"))
		})
	})

	Describe("encoding", func() {
		It("stores Redis-native values without using the cache codec", func() {
			const prefix = "cache:raw:"

			codec := new(cacheCodecSpy)
			cache, err := xredis.NewCache[string](
				client,
				xredis.WithCachePrefix(prefix),
				xredis.WithCacheTTL(time.Minute),
				xredis.WithCacheCodec(codec),
			)
			Expect(err).NotTo(HaveOccurred())

			Expect(cache.Set(ctx, "name", "Ada")).To(Succeed())

			raw, err := client.Raw().Get(ctx, prefix+"name").Result()
			Expect(err).NotTo(HaveOccurred())
			Expect(raw).To(Equal("Ada"))

			value, ok, err := cache.Get(ctx, "name")
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(value).To(Equal("Ada"))
			Expect(codec.marshalCalls.Load()).To(BeZero())
			Expect(codec.unmarshalCalls.Load()).To(BeZero())
		})

		It("uses the cache codec for named scalar types", func() {
			codec := new(cacheCodecSpy)
			cache, err := xredis.NewCache[cacheUserID](
				client,
				xredis.WithCachePrefix("cache:named-scalar:"),
				xredis.WithCacheTTL(time.Minute),
				xredis.WithCacheCodec(codec),
			)
			Expect(err).NotTo(HaveOccurred())

			const expected cacheUserID = 42
			Expect(cache.Set(ctx, "42", expected)).To(Succeed())

			value, ok, err := cache.Get(ctx, "42")
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(value).To(Equal(expected))
			Expect(codec.marshalCalls.Load()).To(Equal(int64(1)))
			Expect(codec.unmarshalCalls.Load()).To(Equal(int64(1)))
		})

		It("uses the cache codec for structured values", func() {
			const prefix = "cache:codec:"

			codec := new(cacheCodecSpy)
			cache, err := xredis.NewCache[cacheUser](
				client,
				xredis.WithCachePrefix(prefix),
				xredis.WithCacheTTL(time.Minute),
				xredis.WithCacheCodec(codec),
			)
			Expect(err).NotTo(HaveOccurred())

			expected := cacheUser{
				ID:     "42",
				Name:   "Ada",
				Active: true,
			}

			Expect(cache.Set(ctx, "42", expected)).To(Succeed())

			raw, err := client.Raw().Get(ctx, prefix+"42").Result()
			Expect(err).NotTo(HaveOccurred())
			Expect(raw).To(MatchJSON(`{"id":"42","name":"Ada","active":true}`))

			value, ok, err := cache.Get(ctx, "42")
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(value).To(Equal(expected))
			Expect(codec.marshalCalls.Load()).To(Equal(int64(1)))
			Expect(codec.unmarshalCalls.Load()).To(Equal(int64(1)))
		})

		It("decodes pointer values without adding another pointer level", func() {
			cache, err := xredis.NewCache[*cacheUser](
				client,
				xredis.WithCachePrefix("cache:pointer:"),
				xredis.WithCacheTTL(time.Minute),
			)
			Expect(err).NotTo(HaveOccurred())

			expected := &cacheUser{
				ID:     "42",
				Name:   "Ada",
				Active: true,
			}

			Expect(cache.Set(ctx, "42", expected)).To(Succeed())

			value, ok, err := cache.Get(ctx, "42")
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(value).To(Equal(expected))
		})

		It("decodes named pointer values", func() {
			cache, err := xredis.NewCache[cacheUserPointer](
				client,
				xredis.WithCachePrefix("cache:named-pointer:"),
				xredis.WithCacheTTL(time.Minute),
			)
			Expect(err).NotTo(HaveOccurred())

			expected := cacheUserPointer(&cacheUser{
				ID:     "42",
				Name:   "Ada",
				Active: true,
			})

			Expect(cache.Set(ctx, "42", expected)).To(Succeed())

			value, ok, err := cache.Get(ctx, "42")
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(value).To(Equal(expected))
		})
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

	Describe("not-found handling", func() {
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

		It("normalizes a custom not-found error without classifying it as a loader failure", func() {
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

			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, xredis.ErrKeyNotFound)).To(BeTrue())
			Expect(errors.Is(err, errUserNotFound)).To(BeTrue())
			Expect(errors.Is(err, xredis.ErrCacheLoad)).To(BeFalse())
		})
	})

	Describe("entry markers", func() {
		It("treats values longer than the default negative marker as regular values", func() {
			cache, err := xredis.NewCache[[]byte](
				client,
				xredis.WithCachePrefix("cache:marker:"),
				xredis.WithCacheTTL(time.Minute),
			)
			Expect(err).NotTo(HaveOccurred())

			expected := []byte{0, 1}
			Expect(cache.Set(ctx, "value", expected)).To(Succeed())

			value, ok, err := cache.Get(ctx, "value")
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(value).To(Equal(expected))
		})

		It("uses a custom negative marker", func() {
			const prefix = "cache:custom-marker:"

			marker := []byte{0x00, 0xff}
			cache, err := xredis.NewCache[cacheUser](
				client,
				xredis.WithCachePrefix(prefix),
				xredis.WithCacheTTL(time.Minute),
				xredis.WithCacheNegativeTTL(time.Minute),
				xredis.WithCacheNegativeMarker(marker),
			)
			Expect(err).NotTo(HaveOccurred())

			var loads atomic.Int64

			loader := func(context.Context) (cacheUser, error) {
				loads.Add(1)

				return cacheUser{}, xredis.ErrKeyNotFound
			}

			_, err = cache.GetOrLoad(ctx, "404", loader)
			Expect(errors.Is(err, xredis.ErrKeyNotFound)).To(BeTrue())

			raw, err := client.Raw().Get(ctx, prefix+"404").Bytes()
			Expect(err).NotTo(HaveOccurred())
			Expect(raw).To(Equal(marker))

			_, err = cache.GetOrLoad(ctx, "404", loader)
			Expect(errors.Is(err, xredis.ErrKeyNotFound)).To(BeTrue())
			Expect(loads.Load()).To(Equal(int64(1)))

			value, ok, err := cache.Get(ctx, "404")
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeFalse())
			Expect(value).To(Equal(cacheUser{}))
		})

		It("treats the default marker as a regular value when a custom marker is configured", func() {
			cache, err := xredis.NewCache[[]byte](
				client,
				xredis.WithCachePrefix("cache:custom-marker-default-value:"),
				xredis.WithCacheTTL(time.Minute),
				xredis.WithCacheNegativeMarker([]byte{0x00, 0xff}),
			)
			Expect(err).NotTo(HaveOccurred())

			expected := []byte{0}
			Expect(cache.Set(ctx, "value", expected)).To(Succeed())

			value, ok, err := cache.Get(ctx, "value")
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(value).To(Equal(expected))
		})

		It("treats values containing the custom marker as regular values", func() {
			marker := []byte{0x00, 0xff}
			cache, err := xredis.NewCache[[]byte](
				client,
				xredis.WithCachePrefix("cache:custom-marker-prefix:"),
				xredis.WithCacheTTL(time.Minute),
				xredis.WithCacheNegativeMarker(marker),
			)
			Expect(err).NotTo(HaveOccurred())

			expected := []byte{0x00, 0xff, 0x01}
			Expect(cache.Set(ctx, "value", expected)).To(Succeed())

			value, ok, err := cache.Get(ctx, "value")
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(value).To(Equal(expected))
		})

		It("copies the configured negative marker", func() {
			const prefix = "cache:custom-marker-copy:"

			marker := []byte{0x00, 0xff}
			cache, err := xredis.NewCache[cacheUser](
				client,
				xredis.WithCachePrefix(prefix),
				xredis.WithCacheTTL(time.Minute),
				xredis.WithCacheNegativeTTL(time.Minute),
				xredis.WithCacheNegativeMarker(marker),
			)
			Expect(err).NotTo(HaveOccurred())

			marker[0] = 0x01

			_, err = cache.GetOrLoad(
				ctx,
				"404",
				func(context.Context) (cacheUser, error) {
					return cacheUser{}, xredis.ErrKeyNotFound
				},
			)
			Expect(errors.Is(err, xredis.ErrKeyNotFound)).To(BeTrue())

			raw, err := client.Raw().Get(ctx, prefix+"404").Bytes()
			Expect(err).NotTo(HaveOccurred())
			Expect(raw).To(Equal([]byte{0x00, 0xff}))
		})

		It("rejects an empty negative marker", func() {
			cache, err := xredis.NewCache[cacheUser](
				client,
				xredis.WithCachePrefix("cache:empty-marker:"),
				xredis.WithCacheTTL(time.Minute),
				xredis.WithCacheNegativeMarker(nil),
			)

			Expect(cache).To(BeNil())
			Expect(errors.Is(err, xredis.ErrInvalidCacheMarker)).To(BeTrue())
		})
	})

	Describe("client compare operations", func() {
		It("operates on positive cache values without a cache-specific format", func() {
			const (
				prefix = "cache:compare:"
				key    = prefix + "42"
			)

			cache, err := xredis.NewCache[string](
				client,
				xredis.WithCachePrefix(prefix),
				xredis.WithCacheTTL(time.Minute),
			)
			Expect(err).NotTo(HaveOccurred())

			Expect(cache.Set(ctx, "42", "processing")).To(Succeed())

			swapped, err := client.CompareAndSwap(
				ctx,
				key,
				"processing",
				"completed",
				time.Minute,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(swapped).To(BeTrue())

			value, ok, err := cache.Get(ctx, "42")
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(value).To(Equal("completed"))

			deleted, err := client.CompareAndDelete(ctx, key, "completed")
			Expect(err).NotTo(HaveOccurred())
			Expect(deleted).To(BeTrue())

			value, ok, err = cache.Get(ctx, "42")
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeFalse())
			Expect(value).To(BeEmpty())
		})
	})

	Describe("loader errors", func() {
		It("classifies loader failures and preserves the original error", func() {
			errLoader := errors.New("database unavailable")

			cache, err := xredis.NewCache[cacheUser](
				client,
				xredis.WithCachePrefix("cache:loader-error:"),
				xredis.WithCacheTTL(time.Minute),
			)
			Expect(err).NotTo(HaveOccurred())

			value, err := cache.GetOrLoad(
				ctx,
				"42",
				func(context.Context) (cacheUser, error) {
					return cacheUser{}, errLoader
				},
			)

			Expect(value).To(Equal(cacheUser{}))
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, xredis.ErrCacheLoad)).To(BeTrue())
			Expect(errors.Is(err, errLoader)).To(BeTrue())
			Expect(errors.Is(err, xredis.ErrKeyNotFound)).To(BeFalse())
		})

		It("does not classify not-found results as loader failures", func() {
			cache, err := xredis.NewCache[cacheUser](
				client,
				xredis.WithCachePrefix("cache:loader-not-found:"),
				xredis.WithCacheTTL(time.Minute),
			)
			Expect(err).NotTo(HaveOccurred())

			value, err := cache.GetOrLoad(
				ctx,
				"404",
				func(context.Context) (cacheUser, error) {
					return cacheUser{}, xredis.ErrKeyNotFound
				},
			)

			Expect(value).To(Equal(cacheUser{}))
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, xredis.ErrKeyNotFound)).To(BeTrue())
			Expect(errors.Is(err, xredis.ErrCacheLoad)).To(BeFalse())
		})
	})

	Describe("strict writes", func() {
		It("returns a cache write error after a successful loader", func() {
			failingClient := newTestClient()
			DeferCleanup(func() {
				_ = failingClient.Close()
			})

			cache, err := xredis.NewCache[cacheUser](
				failingClient,
				xredis.WithCachePrefix("cache:value-write-error:"),
				xredis.WithCacheTTL(time.Minute),
			)
			Expect(err).NotTo(HaveOccurred())

			expected := cacheUser{
				ID:   "42",
				Name: "Ada",
			}

			value, err := cache.GetOrLoad(
				ctx,
				"42",
				func(context.Context) (cacheUser, error) {
					_ = failingClient.Close()

					return expected, nil
				},
			)

			Expect(value).To(Equal(cacheUser{}))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("store cache value"))
			Expect(errors.Is(err, xredis.ErrCacheLoad)).To(BeFalse())
			Expect(errors.Is(err, xredis.ErrKeyNotFound)).To(BeFalse())
		})

		It("returns a cache write error when storing a negative entry fails", func() {
			failingClient := newTestClient()
			DeferCleanup(func() {
				_ = failingClient.Close()
			})

			cache, err := xredis.NewCache[cacheUser](
				failingClient,
				xredis.WithCachePrefix("cache:negative-write-error:"),
				xredis.WithCacheTTL(time.Minute),
				xredis.WithCacheNegativeTTL(time.Minute),
			)
			Expect(err).NotTo(HaveOccurred())

			value, err := cache.GetOrLoad(
				ctx,
				"404",
				func(context.Context) (cacheUser, error) {
					_ = failingClient.Close()

					return cacheUser{}, xredis.ErrKeyNotFound
				},
			)

			Expect(value).To(Equal(cacheUser{}))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("store negative cache entry"))
			Expect(errors.Is(err, xredis.ErrKeyNotFound)).To(BeFalse())
			Expect(errors.Is(err, xredis.ErrCacheLoad)).To(BeFalse())
		})
	})
})
