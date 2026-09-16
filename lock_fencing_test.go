package xredis_test

import (
	"errors"
	"time"

	. "github.com/bsm/ginkgo/v2"
	. "github.com/bsm/gomega"
	"github.com/mkbeh/xredis"
)

var _ = Describe("FencedLock", func() {
	var client *xredis.Client

	BeforeEach(func() {
		client = newTestClient()
		Expect(client.Raw().FlushDB(ctx).Err()).To(Succeed())
	})

	AfterEach(func() {
		Expect(client.Close()).To(Succeed())
	})

	It("uses the provided owner token and exposes fenced lock identity", func() {
		lock, acquired, err := client.TryFencedLockWithToken(
			ctx,
			"lock:{order:42}",
			"fencing:{order:42}",
			"owner-42",
			time.Minute,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(acquired).To(BeTrue())
		Expect(lock.Key()).To(Equal("lock:{order:42}"))
		Expect(lock.Token()).To(Equal("owner-42"))
		Expect(lock.FencingKey()).To(Equal("fencing:{order:42}"))
		Expect(lock.FencingToken()).To(BeNumerically(">", 0))

		Expect(lock.Unlock(ctx)).To(Succeed())
	})

	It("returns monotonically increasing fencing tokens", func() {
		firstLock, acquired, err := client.TryFencedLock(
			ctx,
			"lock:{order:42}",
			"fencing:{order:42}",
			time.Minute,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(acquired).To(BeTrue())

		firstToken := firstLock.FencingToken()
		Expect(firstToken).To(BeNumerically(">", 0))

		Expect(firstLock.Unlock(ctx)).To(Succeed())

		secondLock, acquired, err := client.TryFencedLock(
			ctx,
			"lock:{order:42}",
			"fencing:{order:42}",
			time.Minute,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(acquired).To(BeTrue())
		Expect(secondLock.FencingToken()).To(BeNumerically(">", firstToken))

		Expect(secondLock.Unlock(ctx)).To(Succeed())
	})

	It("does not consume a fencing token when acquisition is contended", func() {
		firstLock, acquired, err := client.TryFencedLock(
			ctx,
			"lock:{order:42}",
			"fencing:{order:42}",
			time.Minute,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(acquired).To(BeTrue())
		firstToken := firstLock.FencingToken()

		contendedLock, acquired, err := client.TryFencedLock(
			ctx,
			"lock:{order:42}",
			"fencing:{order:42}",
			time.Minute,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(acquired).To(BeFalse())
		Expect(contendedLock).To(BeNil())

		Expect(firstLock.Unlock(ctx)).To(Succeed())

		nextLock, acquired, err := client.TryFencedLock(
			ctx,
			"lock:{order:42}",
			"fencing:{order:42}",
			time.Minute,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(acquired).To(BeTrue())
		Expect(nextLock.FencingToken()).To(Equal(firstToken + 1))

		Expect(nextLock.Unlock(ctx)).To(Succeed())
	})

	It("extends a fenced lock owned by the caller", func() {
		lock, acquired, err := client.TryFencedLock(
			ctx,
			"lock:{order:42}",
			"fencing:{order:42}",
			time.Second,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(acquired).To(BeTrue())

		extended, err := lock.Extend(ctx, 5*time.Second)
		Expect(err).NotTo(HaveOccurred())
		Expect(extended).To(BeTrue())

		ttl, err := client.Raw().PTTL(ctx, lock.Key()).Result()
		Expect(err).NotTo(HaveOccurred())
		Expect(ttl).To(BeNumerically(">", 4*time.Second))
		Expect(ttl).To(BeNumerically("<=", 5*time.Second))

		Expect(lock.Unlock(ctx)).To(Succeed())
	})

	It("rejects equal lock and fencing keys", func() {
		lock, acquired, err := client.TryFencedLock(
			ctx,
			"lock:{order:42}",
			"lock:{order:42}",
			time.Minute,
		)

		Expect(errors.Is(err, xredis.ErrInvalidLock)).To(BeTrue())
		Expect(acquired).To(BeFalse())
		Expect(lock).To(BeNil())
	})

	It("rejects TTLs that round to the same Redis millisecond value", func() {
		lock, acquired, err := client.TryFencedLock(
			ctx,
			"lock:{order:42}",
			"fencing:{order:42}",
			1100*time.Microsecond,
			xredis.WithFencingCounterTTL(1200*time.Microsecond),
		)

		Expect(errors.Is(err, xredis.ErrInvalidTTL)).To(BeTrue())
		Expect(acquired).To(BeFalse())
		Expect(lock).To(BeNil())
	})
})
