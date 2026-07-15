package xredis_test

import (
	"errors"
	"time"

	. "github.com/bsm/ginkgo/v2"
	. "github.com/bsm/gomega"
	"github.com/mkbeh/xredis"
)

var _ = Describe("Lock", func() {
	var client *xredis.Client

	BeforeEach(func() {
		client = newTestClient()
		Expect(client.Raw().FlushDB(ctx).Err()).To(Succeed())
	})

	AfterEach(func() {
		Expect(client.Close()).To(Succeed())
	})

	It("does not acquire a lock that is already held", func() {
		firstLock, acquired, err := client.TryLock(
			ctx,
			"lock:order:42",
			time.Minute,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(acquired).To(BeTrue())
		Expect(firstLock).NotTo(BeNil())

		secondLock, acquired, err := client.TryLock(
			ctx,
			"lock:order:42",
			time.Minute,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(acquired).To(BeFalse())
		Expect(secondLock).To(BeNil())

		Expect(firstLock.Unlock(ctx)).To(Succeed())
	})

	It("extends a lock owned by the caller", func() {
		lock, acquired, err := client.TryLock(
			ctx,
			"lock:order:42",
			time.Second,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(acquired).To(BeTrue())

		extended, err := lock.Extend(ctx, 5*time.Second)
		Expect(err).NotTo(HaveOccurred())
		Expect(extended).To(BeTrue())

		ttl, err := client.Raw().PTTL(ctx, "lock:order:42").Result()
		Expect(err).NotTo(HaveOccurred())
		Expect(ttl).To(BeNumerically(">", 4*time.Second))
		Expect(ttl).To(BeNumerically("<=", 5*time.Second))

		Expect(lock.Unlock(ctx)).To(Succeed())
	})

	It("returns ErrLockNotOwned when the lock is unlocked twice", func() {
		lock, acquired, err := client.TryLock(
			ctx,
			"lock:order:42",
			time.Minute,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(acquired).To(BeTrue())

		Expect(lock.Unlock(ctx)).To(Succeed())

		err = lock.Unlock(ctx)
		Expect(errors.Is(err, xredis.ErrLockNotOwned)).To(BeTrue())
	})
})
