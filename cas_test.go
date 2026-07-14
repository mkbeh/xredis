package xredis_test

import (
	"time"

	. "github.com/bsm/ginkgo/v2"
	. "github.com/bsm/gomega"
	"github.com/mkbeh/xredis"
)

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
		It("swaps a matching value and applies a new expiration", func() {
			Expect(client.Set(
				ctx,
				"status",
				"processing",
				5*time.Minute,
			)).To(Succeed())

			swapped, err := client.CompareAndSwap(
				ctx,
				"status",
				"processing",
				"completed",
				time.Hour,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(swapped).To(BeTrue())

			value, ok, err := client.String(ctx, "status")
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(value).To(Equal("completed"))

			ttl, err := client.Raw().PTTL(ctx, "status").Result()
			Expect(err).NotTo(HaveOccurred())
			Expect(ttl).To(BeNumerically(">", 50*time.Minute))
		})

		It("preserves the existing expiration with KeepTTL", func() {
			Expect(client.Set(
				ctx,
				"status",
				"processing",
				2*time.Minute,
			)).To(Succeed())

			before, err := client.Raw().PTTL(ctx, "status").Result()
			Expect(err).NotTo(HaveOccurred())
			Expect(before).To(BeNumerically(">", 0))

			swapped, err := client.CompareAndSwap(
				ctx,
				"status",
				"processing",
				"completed",
				xredis.KeepTTL,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(swapped).To(BeTrue())

			after, err := client.Raw().PTTL(ctx, "status").Result()
			Expect(err).NotTo(HaveOccurred())
			Expect(after).To(BeNumerically(">", 0))
			Expect(after).To(BeNumerically("<=", before))
			Expect(after).To(BeNumerically(">", before-5*time.Second))
		})

		It("removes the existing expiration when expiration is zero", func() {
			Expect(client.Set(
				ctx,
				"status",
				"processing",
				time.Minute,
			)).To(Succeed())

			swapped, err := client.CompareAndSwap(
				ctx,
				"status",
				"processing",
				"completed",
				0,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(swapped).To(BeTrue())

			ttl, err := client.Raw().PTTL(ctx, "status").Result()
			Expect(err).NotTo(HaveOccurred())
			Expect(ttl).To(BeNumerically("<", 0))
		})

		It("rejects a stale expected value without changing the value or expiration", func() {
			Expect(client.Set(
				ctx,
				"status",
				"cancelled",
				time.Minute,
			)).To(Succeed())

			before, err := client.Raw().PTTL(ctx, "status").Result()
			Expect(err).NotTo(HaveOccurred())
			Expect(before).To(BeNumerically(">", 0))

			swapped, err := client.CompareAndSwap(
				ctx,
				"status",
				"processing",
				"completed",
				time.Hour,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(swapped).To(BeFalse())

			value, ok, err := client.String(ctx, "status")
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(value).To(Equal("cancelled"))

			after, err := client.Raw().PTTL(ctx, "status").Result()
			Expect(err).NotTo(HaveOccurred())
			Expect(after).To(BeNumerically(">", 0))
			Expect(after).To(BeNumerically("<=", before))
			Expect(after).To(BeNumerically(">", before-5*time.Second))
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

			deleted, err := client.CompareAndDelete(
				ctx,
				"missing",
				"processing",
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(deleted).To(BeFalse())
		})

		It("rejects an invalid negative expiration without changing the value", func() {
			Expect(client.Set(
				ctx,
				"status",
				"processing",
				time.Minute,
			)).To(Succeed())

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

			ttl, err := client.Raw().PTTL(ctx, "status").Result()
			Expect(err).NotTo(HaveOccurred())
			Expect(ttl).To(BeNumerically(">", 0))
		})

		It("deletes only a matching value", func() {
			Expect(client.Set(
				ctx,
				"status",
				"processing",
				0,
			)).To(Succeed())

			deleted, err := client.CompareAndDelete(
				ctx,
				"status",
				"completed",
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(deleted).To(BeFalse())

			exists, err := client.Exists(ctx, "status")
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeTrue())

			deleted, err = client.CompareAndDelete(
				ctx,
				"status",
				"processing",
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(deleted).To(BeTrue())

			exists, err = client.Exists(ctx, "status")
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeFalse())
		})
	})

	Describe("hash fields", func() {
		It("swaps a matching field and preserves the hash expiration", func() {
			Expect(client.HSet(
				ctx,
				"order:42",
				time.Minute,
				"status",
				"processing",
				"version",
				1,
			)).To(Succeed())

			before, err := client.Raw().PTTL(ctx, "order:42").Result()
			Expect(err).NotTo(HaveOccurred())
			Expect(before).To(BeNumerically(">", 0))

			swapped, err := client.HCompareAndSwap(
				ctx,
				"order:42",
				"status",
				"processing",
				"completed",
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(swapped).To(BeTrue())

			status, ok, err := client.HGet(
				ctx,
				"order:42",
				"status",
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(status).To(Equal("completed"))

			after, err := client.Raw().PTTL(ctx, "order:42").Result()
			Expect(err).NotTo(HaveOccurred())
			Expect(after).To(BeNumerically(">", 0))
			Expect(after).To(BeNumerically("<=", before))
			Expect(after).To(BeNumerically(">", before-5*time.Second))
		})

		It("rejects a stale expected field value", func() {
			Expect(client.HSet(
				ctx,
				"order:42",
				0,
				"status",
				"cancelled",
			)).To(Succeed())

			swapped, err := client.HCompareAndSwap(
				ctx,
				"order:42",
				"status",
				"processing",
				"completed",
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(swapped).To(BeFalse())

			status, ok, err := client.HGet(
				ctx,
				"order:42",
				"status",
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(status).To(Equal("cancelled"))
		})

		It("returns false when the hash or field does not exist", func() {
			swapped, err := client.HCompareAndSwap(
				ctx,
				"missing",
				"status",
				"processing",
				"completed",
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(swapped).To(BeFalse())

			deleted, err := client.HCompareAndDelete(
				ctx,
				"missing",
				"status",
				"processing",
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(deleted).To(BeFalse())

			Expect(client.HSet(
				ctx,
				"order:42",
				0,
				"version",
				1,
			)).To(Succeed())

			swapped, err = client.HCompareAndSwap(
				ctx,
				"order:42",
				"status",
				"processing",
				"completed",
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(swapped).To(BeFalse())

			deleted, err = client.HCompareAndDelete(
				ctx,
				"order:42",
				"status",
				"processing",
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(deleted).To(BeFalse())
		})

		It("deletes only a matching field", func() {
			Expect(client.HSet(
				ctx,
				"order:42",
				0,
				"status",
				"processing",
				"version",
				1,
			)).To(Succeed())

			deleted, err := client.HCompareAndDelete(
				ctx,
				"order:42",
				"status",
				"completed",
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(deleted).To(BeFalse())

			deleted, err = client.HCompareAndDelete(
				ctx,
				"order:42",
				"status",
				"processing",
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(deleted).To(BeTrue())

			_, ok, err := client.HGet(
				ctx,
				"order:42",
				"status",
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeFalse())

			version, ok, err := client.HGet(
				ctx,
				"order:42",
				"version",
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(version).To(Equal("1"))
		})

		It("removes the hash when the last field is deleted", func() {
			Expect(client.HSet(
				ctx,
				"order:42",
				0,
				"status",
				"processing",
			)).To(Succeed())

			deleted, err := client.HCompareAndDelete(
				ctx,
				"order:42",
				"status",
				"processing",
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(deleted).To(BeTrue())

			exists, err := client.Exists(ctx, "order:42")
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeFalse())
		})
	})
})
