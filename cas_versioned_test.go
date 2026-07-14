package xredis_test

import (
	"context"
	"errors"
	"sync"
	"time"

	. "github.com/bsm/ginkgo/v2"
	. "github.com/bsm/gomega"
	"github.com/mkbeh/xredis"
)

type versionedOrder struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Version int64  `json:"version"`
}

type versionedOrderPointer *versionedOrder

type versionedCASResult struct {
	revision xredis.Revision
	status   string
	swapped  bool
	err      error
}

var _ = Describe("VersionedStore", func() {
	const (
		prefix = "versioned:order:"
		key    = "42"
		rawKey = prefix + key
	)

	var (
		client *xredis.Client
		store  *xredis.VersionedStore[versionedOrder]
	)

	BeforeEach(func() {
		client = newTestClient()
		Expect(client.Raw().FlushDB(ctx).Err()).To(Succeed())

		var err error
		store, err = xredis.NewVersionedStore[versionedOrder](
			client,
			xredis.WithVersionedStorePrefix(prefix),
		)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(client.Close()).To(Succeed())
	})

	Describe("construction", func() {
		It("rejects a nil client", func() {
			versionedStore, err := xredis.NewVersionedStore[versionedOrder](nil)

			Expect(versionedStore).To(BeNil())
			Expect(err).To(MatchError(xredis.ErrInvalidVersionedStore))
		})

		It("rejects interface value types", func() {
			versionedStore, err := xredis.NewVersionedStore[any](
				client,
				xredis.WithVersionedStorePrefix("versioned:any:"),
			)

			Expect(versionedStore).To(BeNil())
			Expect(errors.Is(err, xredis.ErrInvalidVersionedStore)).To(BeTrue())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("interface value type"))
		})
	})

	Describe("Get", func() {
		It("returns false when the key does not exist", func() {
			entry, ok, err := store.Get(ctx, key)

			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeFalse())
			Expect(entry).To(Equal(xredis.VersionedValue[versionedOrder]{}))
		})

		It("reads the stored value and revision", func() {
			expected := versionedOrder{
				ID:      "42",
				Status:  "processing",
				Version: 1,
			}

			revision, created, err := store.Create(
				ctx,
				key,
				expected,
				time.Minute,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(created).To(BeTrue())
			Expect(revision).NotTo(BeEmpty())

			entry, ok, err := store.Get(ctx, key)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(entry.Value).To(Equal(expected))
			Expect(entry.Revision).To(Equal(revision))

			raw, err := client.Raw().HGetAll(ctx, rawKey).Result()
			Expect(err).NotTo(HaveOccurred())
			Expect(raw).To(HaveKeyWithValue("revision", string(revision)))
			Expect(raw).To(HaveKey("value"))
			Expect(raw["value"]).To(MatchJSON(
				`{"id":"42","status":"processing","version":1}`,
			))
		})

		It("decodes pointer values", func() {
			pointerStore, err := xredis.NewVersionedStore[*versionedOrder](
				client,
				xredis.WithVersionedStorePrefix("versioned:pointer:"),
			)
			Expect(err).NotTo(HaveOccurred())

			expected := &versionedOrder{
				ID:      "42",
				Status:  "processing",
				Version: 1,
			}

			revision, created, err := pointerStore.Create(
				ctx,
				key,
				expected,
				0,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(created).To(BeTrue())

			entry, ok, err := pointerStore.Get(ctx, key)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(entry.Value).To(Equal(expected))
			Expect(entry.Revision).To(Equal(revision))
		})

		It("decodes named pointer values", func() {
			pointerStore, err := xredis.NewVersionedStore[versionedOrderPointer](
				client,
				xredis.WithVersionedStorePrefix("versioned:named-pointer:"),
			)
			Expect(err).NotTo(HaveOccurred())

			expected := versionedOrderPointer(&versionedOrder{
				ID:      "42",
				Status:  "processing",
				Version: 1,
			})

			revision, created, err := pointerStore.Create(
				ctx,
				key,
				expected,
				0,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(created).To(BeTrue())

			entry, ok, err := pointerStore.Get(ctx, key)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(entry.Value).To(Equal(expected))
			Expect(entry.Revision).To(Equal(revision))
		})

		It("rejects an entry with a missing revision", func() {
			Expect(client.Raw().HSet(
				ctx,
				rawKey,
				"value",
				`{"id":"42","status":"processing","version":1}`,
			).Err()).To(Succeed())

			entry, ok, err := store.Get(ctx, key)

			Expect(entry).To(Equal(xredis.VersionedValue[versionedOrder]{}))
			Expect(ok).To(BeFalse())
			Expect(err).To(MatchError(xredis.ErrInvalidEntry))
		})

		It("rejects an entry with an empty revision", func() {
			Expect(client.Raw().HSet(
				ctx,
				rawKey,
				"value",
				`{"id":"42","status":"processing","version":1}`,
				"revision",
				"",
			).Err()).To(Succeed())

			entry, ok, err := store.Get(ctx, key)

			Expect(entry).To(Equal(xredis.VersionedValue[versionedOrder]{}))
			Expect(ok).To(BeFalse())
			Expect(err).To(MatchError(xredis.ErrInvalidEntry))
		})

		It("returns codec errors for malformed values", func() {
			Expect(client.Raw().HSet(
				ctx,
				rawKey,
				"value",
				"{invalid-json",
				"revision",
				"revision-1",
			).Err()).To(Succeed())

			entry, ok, err := store.Get(ctx, key)

			Expect(entry).To(Equal(xredis.VersionedValue[versionedOrder]{}))
			Expect(ok).To(BeFalse())
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, xredis.ErrInvalidEntry)).To(BeFalse())
		})
	})

	Describe("Create", func() {
		It("creates a value with expiration", func() {
			expected := versionedOrder{
				ID:      "42",
				Status:  "processing",
				Version: 1,
			}

			revision, created, err := store.Create(
				ctx,
				key,
				expected,
				time.Minute,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(created).To(BeTrue())
			Expect(revision).NotTo(BeEmpty())

			ttl, err := client.Raw().PTTL(ctx, rawKey).Result()
			Expect(err).NotTo(HaveOccurred())
			Expect(ttl).To(BeNumerically(">", 0))
			Expect(ttl).To(BeNumerically("<=", time.Minute))
		})

		It("creates a persistent value when expiration is zero", func() {
			revision, created, err := store.Create(
				ctx,
				key,
				versionedOrder{ID: "42"},
				0,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(created).To(BeTrue())
			Expect(revision).NotTo(BeEmpty())

			ttl, err := client.Raw().PTTL(ctx, rawKey).Result()
			Expect(err).NotTo(HaveOccurred())
			Expect(ttl).To(Equal(time.Duration(-1)))
		})

		It("does not overwrite an existing value or expiration", func() {
			original := versionedOrder{
				ID:      "42",
				Status:  "processing",
				Version: 1,
			}
			replacement := versionedOrder{
				ID:      "42",
				Status:  "completed",
				Version: 2,
			}

			originalRevision, created, err := store.Create(
				ctx,
				key,
				original,
				2*time.Minute,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(created).To(BeTrue())

			before, err := client.Raw().PTTL(ctx, rawKey).Result()
			Expect(err).NotTo(HaveOccurred())
			Expect(before).To(BeNumerically(">", 0))

			revision, created, err := store.Create(
				ctx,
				key,
				replacement,
				time.Hour,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(created).To(BeFalse())
			Expect(revision).To(BeEmpty())

			entry, ok, err := store.Get(ctx, key)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(entry.Value).To(Equal(original))
			Expect(entry.Revision).To(Equal(originalRevision))

			after, err := client.Raw().PTTL(ctx, rawKey).Result()
			Expect(err).NotTo(HaveOccurred())
			Expect(after).To(BeNumerically(">", 0))
			Expect(after).To(BeNumerically("<=", before))
			Expect(after).To(BeNumerically(">", before-5*time.Second))
		})

		It("rejects KeepTTL and other negative expirations", func() {
			revision, created, err := store.Create(
				ctx,
				key,
				versionedOrder{ID: "42"},
				xredis.KeepTTL,
			)
			Expect(err).To(MatchError(xredis.ErrInvalidTTL))
			Expect(created).To(BeFalse())
			Expect(revision).To(BeEmpty())

			revision, created, err = store.Create(
				ctx,
				key,
				versionedOrder{ID: "42"},
				-time.Second,
			)
			Expect(err).To(MatchError(xredis.ErrInvalidTTL))
			Expect(created).To(BeFalse())
			Expect(revision).To(BeEmpty())

			exists, err := client.Exists(ctx, rawKey)
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeFalse())
		})

		It("rejects an empty key", func() {
			revision, created, err := store.Create(
				ctx,
				"",
				versionedOrder{},
				0,
			)

			Expect(err).To(MatchError(xredis.ErrInvalidVersionedStore))
			Expect(created).To(BeFalse())
			Expect(revision).To(BeEmpty())
		})
	})

	Describe("CompareAndSwap", func() {
		It("updates a matching revision and applies a new expiration", func() {
			revision, created, err := store.Create(
				ctx,
				key,
				versionedOrder{
					ID:      "42",
					Status:  "processing",
					Version: 1,
				},
				5*time.Minute,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(created).To(BeTrue())

			updated := versionedOrder{
				ID:      "42",
				Status:  "completed",
				Version: 2,
			}

			newRevision, swapped, err := store.CompareAndSwap(
				ctx,
				key,
				revision,
				updated,
				time.Hour,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(swapped).To(BeTrue())
			Expect(newRevision).NotTo(BeEmpty())
			Expect(newRevision).NotTo(Equal(revision))

			entry, ok, err := store.Get(ctx, key)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(entry.Value).To(Equal(updated))
			Expect(entry.Revision).To(Equal(newRevision))

			ttl, err := client.Raw().PTTL(ctx, rawKey).Result()
			Expect(err).NotTo(HaveOccurred())
			Expect(ttl).To(BeNumerically(">", 50*time.Minute))
		})

		It("preserves the existing expiration with KeepTTL", func() {
			revision, created, err := store.Create(
				ctx,
				key,
				versionedOrder{ID: "42", Status: "processing"},
				2*time.Minute,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(created).To(BeTrue())

			before, err := client.Raw().PTTL(ctx, rawKey).Result()
			Expect(err).NotTo(HaveOccurred())
			Expect(before).To(BeNumerically(">", 0))

			newRevision, swapped, err := store.CompareAndSwap(
				ctx,
				key,
				revision,
				versionedOrder{ID: "42", Status: "completed"},
				xredis.KeepTTL,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(swapped).To(BeTrue())
			Expect(newRevision).NotTo(BeEmpty())

			after, err := client.Raw().PTTL(ctx, rawKey).Result()
			Expect(err).NotTo(HaveOccurred())
			Expect(after).To(BeNumerically(">", 0))
			Expect(after).To(BeNumerically("<=", before))
			Expect(after).To(BeNumerically(">", before-5*time.Second))
		})

		It("removes the existing expiration when expiration is zero", func() {
			revision, created, err := store.Create(
				ctx,
				key,
				versionedOrder{ID: "42", Status: "processing"},
				time.Minute,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(created).To(BeTrue())

			newRevision, swapped, err := store.CompareAndSwap(
				ctx,
				key,
				revision,
				versionedOrder{ID: "42", Status: "completed"},
				0,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(swapped).To(BeTrue())
			Expect(newRevision).NotTo(BeEmpty())

			ttl, err := client.Raw().PTTL(ctx, rawKey).Result()
			Expect(err).NotTo(HaveOccurred())
			Expect(ttl).To(Equal(time.Duration(-1)))
		})

		It("rejects a stale revision without changing the value or expiration", func() {
			original := versionedOrder{
				ID:      "42",
				Status:  "processing",
				Version: 1,
			}

			revision, created, err := store.Create(
				ctx,
				key,
				original,
				2*time.Minute,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(created).To(BeTrue())

			before, err := client.Raw().PTTL(ctx, rawKey).Result()
			Expect(err).NotTo(HaveOccurred())

			newRevision, swapped, err := store.CompareAndSwap(
				ctx,
				key,
				xredis.Revision("stale-revision"),
				versionedOrder{
					ID:      "42",
					Status:  "completed",
					Version: 2,
				},
				time.Hour,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(swapped).To(BeFalse())
			Expect(newRevision).To(BeEmpty())

			entry, ok, err := store.Get(ctx, key)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(entry.Value).To(Equal(original))
			Expect(entry.Revision).To(Equal(revision))

			after, err := client.Raw().PTTL(ctx, rawKey).Result()
			Expect(err).NotTo(HaveOccurred())
			Expect(after).To(BeNumerically(">", 0))
			Expect(after).To(BeNumerically("<=", before))
			Expect(after).To(BeNumerically(">", before-5*time.Second))
		})

		It("returns false when the key does not exist", func() {
			revision, swapped, err := store.CompareAndSwap(
				ctx,
				"missing",
				xredis.Revision("revision-1"),
				versionedOrder{ID: "42"},
				time.Minute,
			)

			Expect(err).NotTo(HaveOccurred())
			Expect(swapped).To(BeFalse())
			Expect(revision).To(BeEmpty())
		})

		It("rejects an empty revision and invalid negative expiration", func() {
			revision, swapped, err := store.CompareAndSwap(
				ctx,
				key,
				"",
				versionedOrder{},
				0,
			)
			Expect(err).To(MatchError(xredis.ErrInvalidVersionedStore))
			Expect(swapped).To(BeFalse())
			Expect(revision).To(BeEmpty())

			revision, created, err := store.Create(
				ctx,
				key,
				versionedOrder{ID: "42"},
				time.Minute,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(created).To(BeTrue())

			newRevision, swapped, err := store.CompareAndSwap(
				ctx,
				key,
				revision,
				versionedOrder{ID: "42", Status: "completed"},
				-time.Second,
			)
			Expect(err).To(MatchError(xredis.ErrInvalidTTL))
			Expect(swapped).To(BeFalse())
			Expect(newRevision).To(BeEmpty())
		})

		It("allows only one concurrent update for the same revision", func() {
			revision, created, err := store.Create(
				ctx,
				key,
				versionedOrder{
					ID:      "42",
					Status:  "processing",
					Version: 1,
				},
				time.Minute,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(created).To(BeTrue())

			const callers = 20

			start := make(chan struct{})
			results := make(chan versionedCASResult, callers)

			var wg sync.WaitGroup
			wg.Add(callers)

			for i := range callers {
				go func() {
					defer wg.Done()

					<-start

					status := "completed-" + string(rune('a'+i))
					newRevision, swapped, swapErr := store.CompareAndSwap(
						context.Background(),
						key,
						revision,
						versionedOrder{
							ID:      "42",
							Status:  status,
							Version: int64(i + 2),
						},
						xredis.KeepTTL,
					)

					results <- versionedCASResult{
						revision: newRevision,
						status:   status,
						swapped:  swapped,
						err:      swapErr,
					}
				}()
			}

			close(start)
			wg.Wait()
			close(results)

			successes := 0
			var winner versionedCASResult

			for result := range results {
				Expect(result.err).NotTo(HaveOccurred())

				if result.swapped {
					successes++
					winner = result
				} else {
					Expect(result.revision).To(BeEmpty())
				}
			}

			Expect(successes).To(Equal(1))
			Expect(winner.revision).NotTo(BeEmpty())

			entry, ok, err := store.Get(ctx, key)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(entry.Revision).To(Equal(winner.revision))
			Expect(entry.Value.Status).To(Equal(winner.status))
		})
	})

	Describe("CompareAndDelete", func() {
		It("deletes a value only when the revision matches", func() {
			revision, created, err := store.Create(
				ctx,
				key,
				versionedOrder{ID: "42", Status: "processing"},
				time.Minute,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(created).To(BeTrue())

			deleted, err := store.CompareAndDelete(
				ctx,
				key,
				xredis.Revision("stale-revision"),
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(deleted).To(BeFalse())

			exists, err := client.Exists(ctx, rawKey)
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeTrue())

			deleted, err = store.CompareAndDelete(
				ctx,
				key,
				revision,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(deleted).To(BeTrue())

			exists, err = client.Exists(ctx, rawKey)
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeFalse())
		})

		It("returns false when the key does not exist", func() {
			deleted, err := store.CompareAndDelete(
				ctx,
				"missing",
				xredis.Revision("revision-1"),
			)

			Expect(err).NotTo(HaveOccurred())
			Expect(deleted).To(BeFalse())
		})

		It("rejects an empty revision", func() {
			deleted, err := store.CompareAndDelete(
				ctx,
				key,
				"",
			)

			Expect(err).To(MatchError(xredis.ErrInvalidVersionedStore))
			Expect(deleted).To(BeFalse())
		})
	})
})
