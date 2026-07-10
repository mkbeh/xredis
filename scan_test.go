package xredis_test

import (
	"context"
	"errors"
	"fmt"
	"strings"

	. "github.com/bsm/ginkgo/v2"
	. "github.com/bsm/gomega"
	"github.com/mkbeh/xredis"
	rdb "github.com/redis/go-redis/v9"
)

var _ = Describe("Scan", func() {
	var client *xredis.Client

	BeforeEach(func() {
		client = newTestClient()
		Expect(client.Raw().FlushDB(ctx).Err()).To(Succeed())
	})

	AfterEach(func() {
		Expect(client.Close()).To(Succeed())
	})

	Describe("Scan", func() {
		It("iterates through cursor pages and applies a match pattern", func() {
			expected := make([]string, 0, 128)

			for i := range 128 {
				key := fmt.Sprintf("scan:page:%03d", i)
				expected = append(expected, key)

				Expect(client.Set(ctx, key, i, 0)).To(Succeed())
			}

			Expect(client.Set(ctx, "other:key", "value", 0)).To(Succeed())

			opts := xredis.ScanOptions{
				Match: "scan:page:*",
				Count: 1,
			}

			actual := make([]string, 0, len(expected))
			cursor := uint64(0)

			for {
				opts.Cursor = cursor

				keys, nextCursor, err := client.Scan(ctx, opts)
				Expect(err).NotTo(HaveOccurred())

				for _, key := range keys {
					Expect(strings.HasPrefix(key, "scan:page:")).To(BeTrue())
				}

				actual = append(actual, keys...)
				cursor = nextCursor

				if cursor == 0 {
					break
				}
			}

			Expect(stringSet(actual)).To(Equal(stringSet(expected)))
		})

		It("filters keys by Redis type", func() {
			Expect(client.Set(ctx, "scan:type:string", "value", 0)).To(Succeed())
			Expect(client.HSet(
				ctx,
				"scan:type:hash",
				0,
				"field", "value",
			)).To(Succeed())

			_, err := client.Raw().XAdd(ctx, &rdb.XAddArgs{
				Stream: "scan:type:stream",
				Values: map[string]any{
					"event": "created",
				},
			}).Result()
			Expect(err).NotTo(HaveOccurred())

			stringKeys, err := client.ScanAll(ctx, xredis.ScanOptions{
				Match: "scan:type:*",
				Type:  "string",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(stringSet(stringKeys)).To(Equal(stringSet([]string{
				"scan:type:string",
			})))

			hashKeys, err := client.ScanAll(ctx, xredis.ScanOptions{
				Match: "scan:type:*",
				Type:  "hash",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(stringSet(hashKeys)).To(Equal(stringSet([]string{
				"scan:type:hash",
			})))

			streamKeys, err := client.ScanAll(ctx, xredis.ScanOptions{
				Match: "scan:type:*",
				Type:  "stream",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(stringSet(streamKeys)).To(Equal(stringSet([]string{
				"scan:type:stream",
			})))
		})

		It("rejects a negative count", func() {
			keys, cursor, err := client.Scan(ctx, xredis.ScanOptions{
				Count: -1,
			})

			Expect(errors.Is(err, xredis.ErrInvalidScan)).To(BeTrue())
			Expect(keys).To(BeNil())
			Expect(cursor).To(BeZero())
		})
	})

	Describe("ScanAll", func() {
		It("returns all matching keys and ignores Cursor", func() {
			expected := []string{
				"scan:all:1",
				"scan:all:2",
				"scan:all:3",
			}

			for _, key := range expected {
				Expect(client.Set(ctx, key, "value", 0)).To(Succeed())
			}

			Expect(client.Set(ctx, "scan:other:1", "value", 0)).To(Succeed())

			keys, err := client.ScanAll(ctx, xredis.ScanOptions{
				Cursor: 123456,
				Match:  "scan:all:*",
				Count:  1,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(stringSet(keys)).To(Equal(stringSet(expected)))
		})
	})

	Describe("ScanEach", func() {
		It("calls the handler for every matching key", func() {
			expected := []string{
				"scan:each:1",
				"scan:each:2",
				"scan:each:3",
			}

			for _, key := range expected {
				Expect(client.Set(ctx, key, "value", 0)).To(Succeed())
			}

			var actual []string

			err := client.ScanEach(
				ctx,
				xredis.ScanOptions{
					Cursor: 9876,
					Match:  "scan:each:*",
					Count:  1,
				},
				func(_ context.Context, key string) error {
					actual = append(actual, key)
					return nil
				},
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(stringSet(actual)).To(Equal(stringSet(expected)))
		})

		It("returns handler errors", func() {
			Expect(client.Set(ctx, "scan:handler-error", "value", 0)).
				To(Succeed())

			errHandler := errors.New("scan handler error")

			err := client.ScanEach(
				ctx,
				xredis.ScanOptions{
					Match: "scan:handler-error",
				},
				func(context.Context, string) error {
					return errHandler
				},
			)

			Expect(errors.Is(err, errHandler)).To(BeTrue())
		})

		It("returns context cancellation", func() {
			Expect(client.Set(ctx, "scan:canceled", "value", 0)).To(Succeed())

			canceledCtx, cancel := context.WithCancel(context.Background())
			cancel()

			err := client.ScanEach(
				canceledCtx,
				xredis.ScanOptions{
					Match: "scan:canceled",
				},
				func(context.Context, string) error {
					return nil
				},
			)

			Expect(errors.Is(err, context.Canceled)).To(BeTrue())
		})

		It("rejects a nil handler", func() {
			err := client.ScanEach(ctx, xredis.ScanOptions{}, nil)
			Expect(errors.Is(err, xredis.ErrInvalidScan)).To(BeTrue())
		})
	})

	Describe("ScanEachBatch", func() {
		It("calls the handler for batches and ignores Cursor", func() {
			expected := make([]string, 0, 32)

			for i := range 32 {
				key := fmt.Sprintf("scan:batch:%02d", i)
				expected = append(expected, key)

				Expect(client.Set(ctx, key, "value", 0)).To(Succeed())
			}

			var actual []string
			calls := 0

			err := client.ScanEachBatch(
				ctx,
				xredis.ScanOptions{
					Cursor: 777,
					Match:  "scan:batch:*",
					Count:  2,
				},
				func(_ context.Context, keys []string) error {
					calls++
					actual = append(actual, keys...)

					return nil
				},
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(calls).To(BeNumerically(">", 0))
			Expect(stringSet(actual)).To(Equal(stringSet(expected)))
		})

		It("returns handler errors", func() {
			Expect(client.Set(ctx, "scan:batch-error", "value", 0)).
				To(Succeed())

			errHandler := errors.New("scan batch handler error")

			err := client.ScanEachBatch(
				ctx,
				xredis.ScanOptions{
					Match: "scan:batch-error",
				},
				func(context.Context, []string) error {
					return errHandler
				},
			)

			Expect(errors.Is(err, errHandler)).To(BeTrue())
		})

		It("rejects a nil handler", func() {
			err := client.ScanEachBatch(ctx, xredis.ScanOptions{}, nil)
			Expect(errors.Is(err, xredis.ErrInvalidScan)).To(BeTrue())
		})
	})

	Describe("ScanDelete", func() {
		It("deletes only matching keys", func() {
			for _, key := range []string{
				"scan:delete:1",
				"scan:delete:2",
				"scan:delete:3",
				"scan:keep:1",
			} {
				Expect(client.Set(ctx, key, "value", 0)).To(Succeed())
			}

			Expect(client.ScanDelete(ctx, xredis.ScanOptions{
				Match: "scan:delete:*",
				Count: 1,
			})).To(Succeed())

			for _, key := range []string{
				"scan:delete:1",
				"scan:delete:2",
				"scan:delete:3",
			} {
				exists, err := client.Exists(ctx, key)
				Expect(err).NotTo(HaveOccurred())
				Expect(exists).To(BeFalse())
			}

			exists, err := client.Exists(ctx, "scan:keep:1")
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeTrue())
		})
	})

	Describe("ScanUnlink", func() {
		It("unlinks only matching keys", func() {
			for _, key := range []string{
				"scan:unlink:1",
				"scan:unlink:2",
				"scan:unlink:3",
				"scan:keep:1",
			} {
				Expect(client.Set(ctx, key, "value", 0)).To(Succeed())
			}

			Expect(client.ScanUnlink(ctx, xredis.ScanOptions{
				Match: "scan:unlink:*",
				Count: 1,
			})).To(Succeed())

			for _, key := range []string{
				"scan:unlink:1",
				"scan:unlink:2",
				"scan:unlink:3",
			} {
				exists, err := client.Exists(ctx, key)
				Expect(err).NotTo(HaveOccurred())
				Expect(exists).To(BeFalse())
			}

			exists, err := client.Exists(ctx, "scan:keep:1")
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeTrue())
		})
	})

	It("rejects a nil client", func() {
		var invalidClient *xredis.Client

		keys, cursor, err := invalidClient.Scan(ctx, xredis.ScanOptions{})
		Expect(errors.Is(err, xredis.ErrInvalidScan)).To(BeTrue())
		Expect(keys).To(BeNil())
		Expect(cursor).To(BeZero())

		keys, err = invalidClient.ScanAll(ctx, xredis.ScanOptions{})
		Expect(errors.Is(err, xredis.ErrInvalidScan)).To(BeTrue())
		Expect(keys).To(BeNil())

		err = invalidClient.ScanEach(
			ctx,
			xredis.ScanOptions{},
			func(context.Context, string) error {
				return nil
			},
		)
		Expect(errors.Is(err, xredis.ErrInvalidScan)).To(BeTrue())

		err = invalidClient.ScanEachBatch(
			ctx,
			xredis.ScanOptions{},
			func(context.Context, []string) error {
				return nil
			},
		)
		Expect(errors.Is(err, xredis.ErrInvalidScan)).To(BeTrue())
	})
})

func stringSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))

	for _, value := range values {
		set[value] = struct{}{}
	}

	return set
}
