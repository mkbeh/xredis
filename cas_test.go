package xredis_test

import (
	"time"

	. "github.com/bsm/ginkgo/v2"
	. "github.com/bsm/gomega"
	"github.com/mkbeh/xredis"
)

type casOrder struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Version int64  `json:"version"`
}

var _ = Describe("CAS and CAD", func() {
	var client *xredis.Client

	BeforeEach(func() {
		client = newTestClient()
		Expect(client.Raw().FlushDB(ctx).Err()).To(Succeed())
	})

	AfterEach(func() {
		Expect(client.Close()).To(Succeed())
	})

	Describe("raw values", func() {
		It("swaps a matching value", func() {
			Expect(client.Set(ctx, "status", "processing", time.Minute)).To(Succeed())

			swapped, err := client.CompareAndSwap(
				ctx,
				"status",
				"processing",
				"completed",
				time.Minute,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(swapped).To(BeTrue())

			value, ok, err := client.String(ctx, "status")
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(value).To(Equal("completed"))

			ttl, err := client.Raw().TTL(ctx, "status").Result()
			Expect(err).NotTo(HaveOccurred())
			Expect(ttl).To(BeNumerically(">", 0))
		})

		It("rejects a stale expected value", func() {
			Expect(client.Set(ctx, "status", "cancelled", time.Minute)).To(Succeed())

			swapped, err := client.CompareAndSwap(
				ctx,
				"status",
				"processing",
				"completed",
				time.Minute,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(swapped).To(BeFalse())

			value, ok, err := client.String(ctx, "status")
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(value).To(Equal("cancelled"))
		})

		It("returns false when the key does not exist", func() {
			swapped, err := client.CompareAndSwap(
				ctx,
				"missing",
				"processing",
				"completed",
				time.Minute,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(swapped).To(BeFalse())

			deleted, err := client.CompareAndDelete(ctx, "missing", "processing")
			Expect(err).NotTo(HaveOccurred())
			Expect(deleted).To(BeFalse())
		})

		It("removes the expiration when ttl is zero", func() {
			Expect(client.Set(ctx, "status", "processing", time.Minute)).To(Succeed())

			swapped, err := client.CompareAndSwap(
				ctx,
				"status",
				"processing",
				"completed",
				0,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(swapped).To(BeTrue())

			ttl, err := client.Raw().TTL(ctx, "status").Result()
			Expect(err).NotTo(HaveOccurred())
			Expect(ttl).To(BeNumerically("<", 0))
		})

		It("rejects a negative ttl without changing the value", func() {
			Expect(client.Set(ctx, "status", "processing", time.Minute)).To(Succeed())

			swapped, err := client.CompareAndSwap(
				ctx,
				"status",
				"processing",
				"completed",
				-time.Second,
			)
			Expect(err).To(MatchError(xredis.ErrInvalidTTL))
			Expect(swapped).To(BeFalse())

			value, ok, err := client.String(ctx, "status")
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(value).To(Equal("processing"))
		})

		It("deletes only a matching value", func() {
			Expect(client.Set(ctx, "status", "processing", 0)).To(Succeed())

			deleted, err := client.CompareAndDelete(ctx, "status", "completed")
			Expect(err).NotTo(HaveOccurred())
			Expect(deleted).To(BeFalse())

			exists, err := client.Exists(ctx, "status")
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeTrue())

			deleted, err = client.CompareAndDelete(ctx, "status", "processing")
			Expect(err).NotTo(HaveOccurred())
			Expect(deleted).To(BeTrue())

			exists, err = client.Exists(ctx, "status")
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeFalse())
		})
	})

	Describe("encoded values", func() {
		It("swaps a matching struct", func() {
			oldOrder := casOrder{
				ID:      "42",
				Status:  "processing",
				Version: 1,
			}
			newOrder := casOrder{
				ID:      "42",
				Status:  "completed",
				Version: 2,
			}

			Expect(client.SetStruct(ctx, "order:42", oldOrder, time.Minute)).
				To(Succeed())

			swapped, err := client.CompareAndSwapStruct(
				ctx,
				"order:42",
				oldOrder,
				newOrder,
				time.Minute,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(swapped).To(BeTrue())

			var actual casOrder
			ok, err := client.GetStruct(ctx, "order:42", &actual)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(actual).To(Equal(newOrder))
		})

		It("rejects a stale struct", func() {
			currentOrder := casOrder{
				ID:      "42",
				Status:  "cancelled",
				Version: 2,
			}
			staleOrder := casOrder{
				ID:      "42",
				Status:  "processing",
				Version: 1,
			}
			newOrder := casOrder{
				ID:      "42",
				Status:  "completed",
				Version: 2,
			}

			Expect(client.SetStruct(ctx, "order:42", currentOrder, time.Minute)).
				To(Succeed())

			swapped, err := client.CompareAndSwapStruct(
				ctx,
				"order:42",
				staleOrder,
				newOrder,
				time.Minute,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(swapped).To(BeFalse())

			var actual casOrder
			ok, err := client.GetStruct(ctx, "order:42", &actual)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(actual).To(Equal(currentOrder))
		})

		It("deletes only a matching struct", func() {
			order := casOrder{
				ID:      "42",
				Status:  "processing",
				Version: 1,
			}
			staleOrder := casOrder{
				ID:      "42",
				Status:  "processing",
				Version: 0,
			}

			Expect(client.SetStruct(ctx, "order:42", order, 0)).To(Succeed())

			deleted, err := client.CompareAndDeleteStruct(
				ctx,
				"order:42",
				staleOrder,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(deleted).To(BeFalse())

			deleted, err = client.CompareAndDeleteStruct(
				ctx,
				"order:42",
				order,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(deleted).To(BeTrue())
		})

		It("returns false for a missing encoded value", func() {
			order := casOrder{
				ID:      "42",
				Status:  "processing",
				Version: 1,
			}

			swapped, err := client.CompareAndSwapStruct(
				ctx,
				"missing",
				order,
				order,
				time.Minute,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(swapped).To(BeFalse())

			deleted, err := client.CompareAndDeleteStruct(
				ctx,
				"missing",
				order,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(deleted).To(BeFalse())
		})

		It("rejects a negative ttl", func() {
			oldOrder := casOrder{
				ID:      "42",
				Status:  "processing",
				Version: 1,
			}
			newOrder := casOrder{
				ID:      "42",
				Status:  "completed",
				Version: 2,
			}

			Expect(client.SetStruct(ctx, "order:42", oldOrder, time.Minute)).
				To(Succeed())

			swapped, err := client.CompareAndSwapStruct(
				ctx,
				"order:42",
				oldOrder,
				newOrder,
				-time.Second,
			)
			Expect(err).To(MatchError(xredis.ErrInvalidTTL))
			Expect(swapped).To(BeFalse())
		})

		It("does not mix raw and encoded comparisons", func() {
			Expect(client.SetStruct(ctx, "status", "processing", time.Minute)).
				To(Succeed())

			rawSwapped, err := client.CompareAndSwap(
				ctx,
				"status",
				"processing",
				"completed",
				time.Minute,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(rawSwapped).To(BeFalse())

			structSwapped, err := client.CompareAndSwapStruct(
				ctx,
				"status",
				"processing",
				"completed",
				time.Minute,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(structSwapped).To(BeTrue())
		})
	})
})
