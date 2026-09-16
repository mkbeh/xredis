package xredis_test

import (
	. "github.com/bsm/ginkgo/v2"
	. "github.com/bsm/gomega"
	"github.com/mkbeh/xredis"
)

var _ = Describe("Client constructors", func() {
	It("rejects nil standalone options", func() {
		client, err := xredis.NewClient(nil)

		Expect(client).To(BeNil())
		Expect(err).To(MatchError(xredis.ErrInvalidOptions))
	})

	It("rejects nil cluster options", func() {
		client, err := xredis.NewClusterClient(nil)

		Expect(client).To(BeNil())
		Expect(err).To(MatchError(xredis.ErrInvalidOptions))
	})

	It("rejects nil failover options", func() {
		client, err := xredis.NewFailoverClient(nil)

		Expect(client).To(BeNil())
		Expect(err).To(MatchError(xredis.ErrInvalidOptions))
	})

	It("rejects nil failover cluster options", func() {
		client, err := xredis.NewFailoverClusterClient(nil)

		Expect(client).To(BeNil())
		Expect(err).To(MatchError(xredis.ErrInvalidOptions))
	})

	It("rejects nil ring options", func() {
		client, err := xredis.NewRing(nil)

		Expect(client).To(BeNil())
		Expect(err).To(MatchError(xredis.ErrInvalidOptions))
	})
})
