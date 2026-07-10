package xredis_test

import (
	"errors"
	"time"

	. "github.com/bsm/ginkgo/v2"
	. "github.com/bsm/gomega"
	"github.com/mkbeh/xredis"
)

var errPipelineCodec = errors.New("pipeline codec error")

type pipelineProfile struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

type pipelineHash struct {
	ID     string `redis:"id"`
	Status string `redis:"status"`
}

type failingPipelineCodec struct{}

func (failingPipelineCodec) Marshal(_ any) ([]byte, error) {
	return nil, errPipelineCodec
}

func (failingPipelineCodec) Unmarshal(_ []byte, _ any) error {
	return nil
}

var _ = Describe("Pipeline", func() {
	var client *xredis.Client

	BeforeEach(func() {
		client = newTestClient()
		Expect(client.Raw().FlushDB(ctx).Err()).To(Succeed())
	})

	AfterEach(func() {
		Expect(client.Close()).To(Succeed())
	})

	Describe("SetMany", func() {
		It("stores multiple raw Redis values", func() {
			err := client.SetMany(ctx, []xredis.SetItem{
				{
					Key:        "raw:string",
					Value:      "hello",
					Expiration: time.Minute,
				},
				{
					Key:        "raw:int",
					Value:      42,
					Expiration: time.Minute,
				},
				{
					Key:        "raw:bytes",
					Value:      []byte("payload"),
					Expiration: time.Minute,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			stringValue, ok, err := client.String(ctx, "raw:string")
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(stringValue).To(Equal("hello"))

			intValue, ok, err := client.Int(ctx, "raw:int")
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(intValue).To(Equal(42))

			bytesValue, ok, err := client.Bytes(ctx, "raw:bytes")
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(bytesValue).To(Equal([]byte("payload")))

			for _, key := range []string{"raw:string", "raw:int", "raw:bytes"} {
				ttl, ttlErr := client.Raw().TTL(ctx, key).Result()
				Expect(ttlErr).NotTo(HaveOccurred())
				Expect(ttl).To(BeNumerically(">", 0))
				Expect(ttl).To(BeNumerically("<=", time.Minute))
			}
		})

		It("stores values without expiration when ttl is zero", func() {
			Expect(client.SetMany(ctx, []xredis.SetItem{
				{
					Key:   "raw:persistent",
					Value: "value",
				},
			})).To(Succeed())

			ttl, err := client.Raw().TTL(ctx, "raw:persistent").Result()
			Expect(err).NotTo(HaveOccurred())
			Expect(ttl).To(Equal(time.Duration(-1)))
		})

		It("is compatible with raw compare operations", func() {
			Expect(client.SetMany(ctx, []xredis.SetItem{
				{
					Key:   "raw:status",
					Value: "processing",
				},
			})).To(Succeed())

			deleted, err := client.CompareAndDelete(
				ctx,
				"raw:status",
				"processing",
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(deleted).To(BeTrue())
		})

		It("does nothing for an empty item list", func() {
			Expect(client.SetMany(ctx, nil)).To(Succeed())
			Expect(client.SetMany(ctx, []xredis.SetItem{})).To(Succeed())
		})

		It("rejects a negative ttl without executing queued commands", func() {
			err := client.SetMany(ctx, []xredis.SetItem{
				{
					Key:   "raw:valid",
					Value: "value",
				},
				{
					Key:        "raw:invalid",
					Value:      "value",
					Expiration: -time.Second,
				},
			})
			Expect(err).To(MatchError(xredis.ErrInvalidTTL))

			exists, err := client.Exists(ctx, "raw:valid")
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeFalse())

			exists, err = client.Exists(ctx, "raw:invalid")
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeFalse())
		})
	})

	Describe("SetStructMany", func() {
		It("encodes and stores multiple values with the client codec", func() {
			profile := pipelineProfile{
				ID:     "42",
				Name:   "Ada",
				Active: true,
			}

			Expect(client.SetStructMany(ctx, []xredis.SetItem{
				{
					Key:        "encoded:profile",
					Value:      profile,
					Expiration: time.Minute,
				},
				{
					Key:   "encoded:status",
					Value: "processing",
				},
			})).To(Succeed())

			var actual pipelineProfile
			ok, err := client.GetStruct(ctx, "encoded:profile", &actual)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(actual).To(Equal(profile))

			var status string
			ok, err = client.GetStruct(ctx, "encoded:status", &status)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(status).To(Equal("processing"))

			ttl, err := client.Raw().TTL(ctx, "encoded:profile").Result()
			Expect(err).NotTo(HaveOccurred())
			Expect(ttl).To(BeNumerically(">", 0))

			ttl, err = client.Raw().TTL(ctx, "encoded:status").Result()
			Expect(err).NotTo(HaveOccurred())
			Expect(ttl).To(Equal(time.Duration(-1)))
		})

		It("does not mix encoded values with raw comparisons", func() {
			Expect(client.SetStructMany(ctx, []xredis.SetItem{
				{
					Key:   "encoded:status",
					Value: "processing",
				},
			})).To(Succeed())

			rawDeleted, err := client.CompareAndDelete(
				ctx,
				"encoded:status",
				"processing",
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(rawDeleted).To(BeFalse())

			exists, err := client.Exists(ctx, "encoded:status")
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeTrue())

			encodedDeleted, err := client.CompareAndDeleteStruct(
				ctx,
				"encoded:status",
				"processing",
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(encodedDeleted).To(BeTrue())
		})

		It("returns codec errors without executing queued commands", func() {
			codecClient, err := xredis.NewClient(
				xredis.WithClientConfig(&xredis.ClientConfig{
					Addr: redisAddr,
					DB:   testDB,
				}),
				xredis.WithClientID("xredis-pipeline-codec-test"),
				xredis.WithCodec(failingPipelineCodec{}),
			)
			Expect(err).NotTo(HaveOccurred())
			defer func() {
				Expect(codecClient.Close()).To(Succeed())
			}()

			err = codecClient.SetStructMany(ctx, []xredis.SetItem{
				{
					Key:   "encoded:first",
					Value: "first",
				},
				{
					Key:   "encoded:second",
					Value: "second",
				},
			})
			Expect(err).To(MatchError(errPipelineCodec))

			exists, err := client.Exists(ctx, "encoded:first")
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeFalse())

			exists, err = client.Exists(ctx, "encoded:second")
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeFalse())
		})

		It("rejects a negative ttl without writing values", func() {
			err := client.SetStructMany(ctx, []xredis.SetItem{
				{
					Key:   "encoded:valid",
					Value: pipelineProfile{ID: "42"},
				},
				{
					Key:        "encoded:invalid",
					Value:      pipelineProfile{ID: "7"},
					Expiration: -time.Second,
				},
			})
			Expect(err).To(MatchError(xredis.ErrInvalidTTL))

			exists, err := client.Exists(ctx, "encoded:valid")
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeFalse())

			exists, err = client.Exists(ctx, "encoded:invalid")
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeFalse())
		})

		It("does nothing for an empty item list", func() {
			Expect(client.SetStructMany(ctx, nil)).To(Succeed())
			Expect(client.SetStructMany(ctx, []xredis.SetItem{})).To(Succeed())
		})
	})

	Describe("HSetMany", func() {
		It("sets fields in multiple hashes", func() {
			Expect(client.HSetMany(ctx, []xredis.HSetItem{
				{
					Key:        "hash:user:42",
					Values:     []any{"name", "Ada", "age", 36},
					Expiration: time.Minute,
				},
				{
					Key: "hash:order:7",
					Values: []any{
						pipelineHash{
							ID:     "7",
							Status: "processing",
						},
					},
				},
			})).To(Succeed())

			name, ok, err := client.HGet(ctx, "hash:user:42", "name")
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(name).To(Equal("Ada"))

			age, ok, err := client.HGet(ctx, "hash:user:42", "age")
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(age).To(Equal("36"))

			var order pipelineHash
			ok, err = client.HGetAll(ctx, "hash:order:7", &order)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(order).To(Equal(pipelineHash{
				ID:     "7",
				Status: "processing",
			}))

			ttl, err := client.Raw().TTL(ctx, "hash:user:42").Result()
			Expect(err).NotTo(HaveOccurred())
			Expect(ttl).To(BeNumerically(">", 0))

			ttl, err = client.Raw().TTL(ctx, "hash:order:7").Result()
			Expect(err).NotTo(HaveOccurred())
			Expect(ttl).To(Equal(time.Duration(-1)))
		})

		It("leaves an existing expiration unchanged when ttl is zero", func() {
			Expect(client.HSet(
				ctx,
				"hash:user:42",
				time.Minute,
				"name", "Ada",
			)).To(Succeed())

			Expect(client.HSetMany(ctx, []xredis.HSetItem{
				{
					Key:        "hash:user:42",
					Values:     []any{"age", 36},
					Expiration: 0,
				},
			})).To(Succeed())

			ttl, err := client.Raw().TTL(ctx, "hash:user:42").Result()
			Expect(err).NotTo(HaveOccurred())
			Expect(ttl).To(BeNumerically(">", 0))
		})

		It("rejects empty hash values without executing queued commands", func() {
			err := client.HSetMany(ctx, []xredis.HSetItem{
				{
					Key:    "hash:valid",
					Values: []any{"field", "value"},
				},
				{
					Key:    "hash:invalid",
					Values: nil,
				},
			})
			Expect(err).To(MatchError(xredis.ErrInvalidHashObject))

			exists, err := client.Exists(ctx, "hash:valid")
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeFalse())

			exists, err = client.Exists(ctx, "hash:invalid")
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeFalse())
		})

		It("rejects a negative ttl without writing hashes", func() {
			err := client.HSetMany(ctx, []xredis.HSetItem{
				{
					Key:    "hash:valid",
					Values: []any{"field", "value"},
				},
				{
					Key:        "hash:invalid",
					Values:     []any{"field", "value"},
					Expiration: -time.Second,
				},
			})
			Expect(err).To(MatchError(xredis.ErrInvalidTTL))

			exists, err := client.Exists(ctx, "hash:valid")
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeFalse())
		})

		It("does nothing for an empty item list", func() {
			Expect(client.HSetMany(ctx, nil)).To(Succeed())
			Expect(client.HSetMany(ctx, []xredis.HSetItem{})).To(Succeed())
		})
	})

	Describe("DeleteMany", func() {
		It("deletes multiple existing keys and ignores missing keys", func() {
			Expect(client.SetMany(ctx, []xredis.SetItem{
				{Key: "delete:1", Value: "one"},
				{Key: "delete:2", Value: "two"},
			})).To(Succeed())

			Expect(client.DeleteMany(ctx, []string{
				"delete:1",
				"delete:2",
				"delete:missing",
			})).To(Succeed())

			exists, err := client.Exists(ctx, "delete:1")
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeFalse())

			exists, err = client.Exists(ctx, "delete:2")
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeFalse())
		})

		It("does nothing for an empty key list", func() {
			Expect(client.DeleteMany(ctx, nil)).To(Succeed())
			Expect(client.DeleteMany(ctx, []string{})).To(Succeed())
		})
	})

	Describe("UnlinkMany", func() {
		It("unlinks multiple existing keys and ignores missing keys", func() {
			Expect(client.SetMany(ctx, []xredis.SetItem{
				{Key: "unlink:1", Value: "one"},
				{Key: "unlink:2", Value: "two"},
			})).To(Succeed())

			Expect(client.UnlinkMany(ctx, []string{
				"unlink:1",
				"unlink:2",
				"unlink:missing",
			})).To(Succeed())

			exists, err := client.Exists(ctx, "unlink:1")
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeFalse())

			exists, err = client.Exists(ctx, "unlink:2")
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeFalse())
		})

		It("does nothing for an empty key list", func() {
			Expect(client.UnlinkMany(ctx, nil)).To(Succeed())
			Expect(client.UnlinkMany(ctx, []string{})).To(Succeed())
		})
	})

	It("rejects a nil client", func() {
		var invalidClient *xredis.Client

		Expect(invalidClient.SetMany(ctx, nil)).
			To(MatchError(xredis.ErrInvalidPipeline))
		Expect(invalidClient.SetStructMany(ctx, nil)).
			To(MatchError(xredis.ErrInvalidPipeline))
		Expect(invalidClient.HSetMany(ctx, nil)).
			To(MatchError(xredis.ErrInvalidPipeline))
		Expect(invalidClient.DeleteMany(ctx, nil)).
			To(MatchError(xredis.ErrInvalidPipeline))
		Expect(invalidClient.UnlinkMany(ctx, nil)).
			To(MatchError(xredis.ErrInvalidPipeline))
	})
})
