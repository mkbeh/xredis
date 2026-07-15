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
