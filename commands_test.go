package xredis_test

import (
	"time"

	. "github.com/bsm/ginkgo/v2"
	. "github.com/bsm/gomega"
	"github.com/mkbeh/xredis"
)

type testProfile struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

type testUserHash struct {
	ID      string `redis:"id"`
	Name    string `redis:"name"`
	Age     int    `redis:"age"`
	Ignored string `redis:"-"`
}

var _ = Describe("Commands", func() {
	var client *xredis.Client

	BeforeEach(func() {
		client = newTestClient()
		Expect(client.Raw().FlushDB(ctx).Err()).To(Succeed())
	})

	AfterEach(func() {
		Expect(client.Close()).To(Succeed())
	})

	Describe("keys", func() {
		It("checks whether a key exists", func() {
			exists, err := client.Exists(ctx, "key")
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeFalse())

			Expect(client.Set(ctx, "key", "value", 0)).To(Succeed())

			exists, err = client.Exists(ctx, "key")
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeTrue())
		})

		It("deletes a key", func() {
			Expect(client.Set(ctx, "key", "value", 0)).To(Succeed())
			Expect(client.Delete(ctx, "key")).To(Succeed())
			Expect(client.Delete(ctx, "key")).To(Succeed())

			exists, err := client.Exists(ctx, "key")
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeFalse())
		})
	})

	Describe("strings", func() {
		It("sets and gets a raw value", func() {
			Expect(client.Set(ctx, "message", "hello", time.Minute)).To(Succeed())

			var value string
			ok, err := client.Get(ctx, "message", &value)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(value).To(Equal("hello"))

			ttl, err := client.Raw().TTL(ctx, "message").Result()
			Expect(err).NotTo(HaveOccurred())
			Expect(ttl).To(BeNumerically(">", 0))
		})

		It("returns ok=false for a missing raw value", func() {
			var value string
			ok, err := client.Get(ctx, "missing", &value)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeFalse())
			Expect(value).To(BeEmpty())
		})

		It("rejects a negative SET TTL", func() {
			err := client.Set(ctx, "key", "value", -time.Second)
			Expect(err).To(MatchError(xredis.ErrInvalidTTL))
		})

		It("sets a value only when the key does not exist", func() {
			ok, err := client.SetNX(ctx, "key", "first", time.Minute)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())

			ok, err = client.SetNX(ctx, "key", "second", time.Minute)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeFalse())

			value, exists, err := client.String(ctx, "key")
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeTrue())
			Expect(value).To(Equal("first"))
		})

		It("sets a value only when the key exists", func() {
			ok, err := client.SetXX(ctx, "key", "first", time.Minute)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeFalse())

			Expect(client.Set(ctx, "key", "first", time.Minute)).To(Succeed())

			ok, err = client.SetXX(ctx, "key", "second", time.Minute)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())

			value, exists, err := client.String(ctx, "key")
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeTrue())
			Expect(value).To(Equal("second"))
		})

		It("gets and deletes a value atomically", func() {
			Expect(client.Set(ctx, "key", "value", 0)).To(Succeed())

			value, ok, err := client.GetDel(ctx, "key")
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(value).To(Equal("value"))

			_, ok, err = client.GetDel(ctx, "key")
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeFalse())
		})

		It("gets a value and updates its expiration atomically", func() {
			Expect(client.Set(ctx, "key", "value", 0)).To(Succeed())

			value, ok, err := client.GetEx(ctx, "key", time.Minute)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(value).To(Equal("value"))

			ttl, err := client.Raw().TTL(ctx, "key").Result()
			Expect(err).NotTo(HaveOccurred())
			Expect(ttl).To(BeNumerically(">", 0))

			value, ok, err = client.GetEx(ctx, "key", 0)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(value).To(Equal("value"))

			ttl, err = client.Raw().TTL(ctx, "key").Result()
			Expect(err).NotTo(HaveOccurred())
			Expect(ttl).To(BeNumerically("<", 0))
		})

		It("rejects a negative GETEX TTL", func() {
			_, _, err := client.GetEx(ctx, "key", -time.Second)
			Expect(err).To(MatchError(xredis.ErrInvalidTTL))
		})
	})

	Describe("encoded values", func() {
		It("sets and gets a struct", func() {
			expected := testProfile{
				ID:     "42",
				Name:   "Ada",
				Active: true,
			}

			Expect(client.SetStruct(ctx, "profile", expected, time.Minute)).To(Succeed())

			var actual testProfile
			ok, err := client.GetStruct(ctx, "profile", &actual)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(actual).To(Equal(expected))
		})

		It("returns ok=false for a missing struct", func() {
			var profile testProfile
			ok, err := client.GetStruct(ctx, "missing", &profile)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeFalse())
			Expect(profile).To(Equal(testProfile{}))
		})

		It("supports conditional struct writes", func() {
			first := testProfile{ID: "42", Name: "Ada"}
			second := testProfile{ID: "42", Name: "Grace"}

			ok, err := client.SetStructNX(ctx, "profile", first, time.Minute)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())

			ok, err = client.SetStructNX(ctx, "profile", second, time.Minute)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeFalse())

			ok, err = client.SetStructXX(ctx, "profile", second, time.Minute)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())

			var actual testProfile
			ok, err = client.GetStruct(ctx, "profile", &actual)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(actual).To(Equal(second))
		})

		It("gets and deletes a struct atomically", func() {
			expected := testProfile{ID: "42", Name: "Ada"}
			Expect(client.SetStruct(ctx, "profile", expected, 0)).To(Succeed())

			var actual testProfile
			ok, err := client.GetStructDel(ctx, "profile", &actual)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(actual).To(Equal(expected))

			ok, err = client.GetStruct(ctx, "profile", &actual)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeFalse())
		})

		It("gets a struct and updates its expiration atomically", func() {
			expected := testProfile{ID: "42", Name: "Ada"}
			Expect(client.SetStruct(ctx, "profile", expected, 0)).To(Succeed())

			var actual testProfile
			ok, err := client.GetStructEx(ctx, "profile", &actual, time.Minute)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(actual).To(Equal(expected))

			ttl, err := client.Raw().TTL(ctx, "profile").Result()
			Expect(err).NotTo(HaveOccurred())
			Expect(ttl).To(BeNumerically(">", 0))
		})

		It("rejects negative TTLs for encoded writes", func() {
			profile := testProfile{ID: "42"}

			Expect(client.SetStruct(ctx, "profile", profile, -time.Second)).
				To(MatchError(xredis.ErrInvalidTTL))

			_, err := client.SetStructNX(ctx, "profile", profile, -time.Second)
			Expect(err).To(MatchError(xredis.ErrInvalidTTL))

			_, err = client.SetStructXX(ctx, "profile", profile, -time.Second)
			Expect(err).To(MatchError(xredis.ErrInvalidTTL))
		})
	})

	Describe("hashes", func() {
		It("sets, reads, and deletes hash fields", func() {
			Expect(client.HSet(
				ctx,
				"user:42",
				time.Minute,
				"name", "Ada",
				"age", 36,
			)).To(Succeed())

			name, ok, err := client.HGet(ctx, "user:42", "name")
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(name).To(Equal("Ada"))

			exists, err := client.HExists(ctx, "user:42", "age")
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeTrue())

			deleted, err := client.HDel(ctx, "user:42", "age", "missing")
			Expect(err).NotTo(HaveOccurred())
			Expect(deleted).To(Equal(int64(1)))

			exists, err = client.HExists(ctx, "user:42", "age")
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeFalse())

			ttl, err := client.Raw().TTL(ctx, "user:42").Result()
			Expect(err).NotTo(HaveOccurred())
			Expect(ttl).To(BeNumerically(">", 0))
		})

		It("sets a hash from a redis-tagged struct", func() {
			expected := testUserHash{
				ID:      "42",
				Name:    "Ada",
				Age:     36,
				Ignored: "ignored",
			}

			Expect(client.HSet(ctx, "user:42", 0, expected)).To(Succeed())

			var actual testUserHash
			ok, err := client.HGetAll(ctx, "user:42", &actual)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(actual.ID).To(Equal(expected.ID))
			Expect(actual.Name).To(Equal(expected.Name))
			Expect(actual.Age).To(Equal(expected.Age))
			Expect(actual.Ignored).To(BeEmpty())

			exists, err := client.HExists(ctx, "user:42", "Ignored")
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeFalse())
		})

		It("returns ok=false for missing hashes and fields", func() {
			var user testUserHash
			ok, err := client.HGetAll(ctx, "missing", &user)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeFalse())

			value, ok, err := client.HGet(ctx, "missing", "name")
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeFalse())
			Expect(value).To(BeEmpty())

			exists, err := client.HExists(ctx, "missing", "name")
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeFalse())
		})

		It("increments a hash field and returns the updated value", func() {
			value, err := client.HIncrBy(ctx, "user:42", "views", 2)
			Expect(err).NotTo(HaveOccurred())
			Expect(value).To(Equal(int64(2)))

			value, err = client.HIncrBy(ctx, "user:42", "views", 3)
			Expect(err).NotTo(HaveOccurred())
			Expect(value).To(Equal(int64(5)))
		})

		It("leaves an existing hash expiration unchanged when ttl is zero", func() {
			Expect(client.HSet(
				ctx,
				"user:42",
				time.Minute,
				"name", "Ada",
			)).To(Succeed())

			Expect(client.HSet(
				ctx,
				"user:42",
				0,
				"age", 36,
			)).To(Succeed())

			ttl, err := client.Raw().TTL(ctx, "user:42").Result()
			Expect(err).NotTo(HaveOccurred())
			Expect(ttl).To(BeNumerically(">", 0))
		})

		It("validates hash arguments", func() {
			Expect(client.HSet(ctx, "user:42", -time.Second, "name", "Ada")).
				To(MatchError(xredis.ErrInvalidTTL))

			Expect(client.HSet(ctx, "user:42", 0)).
				To(MatchError(xredis.ErrInvalidHashObject))

			ok, err := client.HGetAll(ctx, "user:42", nil)
			Expect(err).To(MatchError(xredis.ErrInvalidHashObject))
			Expect(ok).To(BeFalse())
		})
	})

	Describe("scalar getters", func() {
		It("reads supported scalar types", func() {
			Expect(client.Set(ctx, "bool", true, 0)).To(Succeed())
			Expect(client.Set(ctx, "bytes", []byte("hello"), 0)).To(Succeed())
			Expect(client.Set(ctx, "float64", 3.14, 0)).To(Succeed())
			Expect(client.Set(ctx, "int", 42, 0)).To(Succeed())
			Expect(client.Set(ctx, "int64", int64(43), 0)).To(Succeed())
			Expect(client.Set(ctx, "uint64", uint64(44), 0)).To(Succeed())
			Expect(client.Set(ctx, "string", "xredis", 0)).To(Succeed())

			boolValue, ok, err := client.Bool(ctx, "bool")
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(boolValue).To(BeTrue())

			bytesValue, ok, err := client.Bytes(ctx, "bytes")
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(bytesValue).To(Equal([]byte("hello")))

			floatValue, ok, err := client.Float64(ctx, "float64")
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(floatValue).To(Equal(3.14))

			intValue, ok, err := client.Int(ctx, "int")
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(intValue).To(Equal(42))

			int64Value, ok, err := client.Int64(ctx, "int64")
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(int64Value).To(Equal(int64(43)))

			uint64Value, ok, err := client.Uint64(ctx, "uint64")
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(uint64Value).To(Equal(uint64(44)))

			stringValue, ok, err := client.String(ctx, "string")
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(stringValue).To(Equal("xredis"))
		})

		It("returns ok=false for a missing scalar value", func() {
			value, ok, err := client.String(ctx, "missing")
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeFalse())
			Expect(value).To(BeEmpty())
		})

		It("returns conversion errors", func() {
			Expect(client.Set(ctx, "key", "not-an-integer", 0)).To(Succeed())

			_, ok, err := client.Int64(ctx, "key")
			Expect(err).To(HaveOccurred())
			Expect(ok).To(BeFalse())
		})
	})

	Describe("counters", func() {
		It("increments and decrements a counter and returns updated values", func() {
			value, err := client.Incr(ctx, "counter")
			Expect(err).NotTo(HaveOccurred())
			Expect(value).To(Equal(int64(1)))

			value, err = client.Incr(ctx, "counter")
			Expect(err).NotTo(HaveOccurred())
			Expect(value).To(Equal(int64(2)))

			value, err = client.Decr(ctx, "counter")
			Expect(err).NotTo(HaveOccurred())
			Expect(value).To(Equal(int64(1)))
		})

		It("returns an error for a non-integer counter", func() {
			Expect(client.Set(ctx, "counter", "invalid", 0)).To(Succeed())

			_, err := client.Incr(ctx, "counter")
			Expect(err).To(HaveOccurred())

			_, err = client.Decr(ctx, "counter")
			Expect(err).To(HaveOccurred())
		})
	})
})
