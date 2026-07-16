package xredis

import (
	"runtime"
	"testing"
)

type benchmarkCacheUser struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

type benchmarkCacheUserPointer *benchmarkCacheUser

var (
	benchmarkCacheUserValueSink    benchmarkCacheUser
	benchmarkCacheUserPointerSink  *benchmarkCacheUser
	benchmarkCacheNamedPointerSink benchmarkCacheUserPointer
)

// Direct value decoding without generics or reflection.
func decodeCacheUserValue(
	codec Codec,
	data []byte,
) (benchmarkCacheUser, error) {
	var value benchmarkCacheUser

	if err := codec.Unmarshal(data, &value); err != nil {
		return benchmarkCacheUser{}, err
	}

	return value, nil
}

// Direct pointer decoding without generics or reflection.
func decodeCacheUserPointer(
	codec Codec,
	data []byte,
) (*benchmarkCacheUser, error) {
	value := new(benchmarkCacheUser)

	if err := codec.Unmarshal(data, value); err != nil {
		return nil, err
	}

	return value, nil
}

// Direct named-pointer decoding without generics or reflection.
func decodeCacheUserNamedPointer(
	codec Codec,
	data []byte,
) (benchmarkCacheUserPointer, error) {
	value := new(benchmarkCacheUser)

	if err := codec.Unmarshal(data, value); err != nil {
		return nil, err
	}

	return benchmarkCacheUserPointer(value), nil
}

// decodeCacheValueLegacy preserves the previous generic implementation as a
// benchmark baseline. For pointer T, the codec receives **T.
func decodeCacheValueLegacy[T any](
	codec Codec,
	data []byte,
) (T, error) {
	var value T

	if err := codec.Unmarshal(data, &value); err != nil {
		var zero T
		return zero, err
	}

	return value, nil
}

func BenchmarkDecodeCacheValue(b *testing.B) {
	codec := JSONCodec{}
	data := []byte(`{"id":"42","name":"Ada","active":true}`)

	valueCache := &Cache[benchmarkCacheUser]{codec: codec}
	pointerCache := &Cache[*benchmarkCacheUser]{codec: codec}
	namedPointerCache := &Cache[benchmarkCacheUserPointer]{codec: codec}

	b.Run("value/direct", func(b *testing.B) {
		b.ReportAllocs()

		for b.Loop() {
			value, err := decodeCacheUserValue(codec, data)
			if err != nil {
				b.Fatal(err)
			}

			benchmarkCacheUserValueSink = value
		}
	})

	b.Run("value/legacy", func(b *testing.B) {
		b.ReportAllocs()

		for b.Loop() {
			value, err := decodeCacheValueLegacy[benchmarkCacheUser](
				codec,
				data,
			)
			if err != nil {
				b.Fatal(err)
			}

			benchmarkCacheUserValueSink = value
		}
	})

	b.Run("value/current", func(b *testing.B) {
		b.ReportAllocs()

		for b.Loop() {
			value, err := valueCache.decode(nil, data)
			if err != nil {
				b.Fatal(err)
			}

			benchmarkCacheUserValueSink = value
		}
	})

	b.Run("pointer/direct", func(b *testing.B) {
		b.ReportAllocs()

		for b.Loop() {
			value, err := decodeCacheUserPointer(codec, data)
			if err != nil {
				b.Fatal(err)
			}

			benchmarkCacheUserPointerSink = value
		}
	})

	b.Run("pointer/legacy", func(b *testing.B) {
		b.ReportAllocs()

		for b.Loop() {
			value, err := decodeCacheValueLegacy[*benchmarkCacheUser](
				codec,
				data,
			)
			if err != nil {
				b.Fatal(err)
			}

			benchmarkCacheUserPointerSink = value
		}
	})

	b.Run("pointer/current", func(b *testing.B) {
		b.ReportAllocs()

		for b.Loop() {
			value, err := pointerCache.decode(nil, data)
			if err != nil {
				b.Fatal(err)
			}

			benchmarkCacheUserPointerSink = value
		}
	})

	b.Run("named_pointer/direct", func(b *testing.B) {
		b.ReportAllocs()

		for b.Loop() {
			value, err := decodeCacheUserNamedPointer(codec, data)
			if err != nil {
				b.Fatal(err)
			}

			benchmarkCacheNamedPointerSink = value
		}
	})

	b.Run("named_pointer/legacy", func(b *testing.B) {
		b.ReportAllocs()

		for b.Loop() {
			value, err := decodeCacheValueLegacy[benchmarkCacheUserPointer](
				codec,
				data,
			)
			if err != nil {
				b.Fatal(err)
			}

			benchmarkCacheNamedPointerSink = value
		}
	})

	b.Run("named_pointer/current", func(b *testing.B) {
		b.ReportAllocs()

		for b.Loop() {
			value, err := namedPointerCache.decode(nil, data)
			if err != nil {
				b.Fatal(err)
			}

			benchmarkCacheNamedPointerSink = value
		}
	})
}

func BenchmarkDecodeCacheValueParallel(b *testing.B) {
	codec := JSONCodec{}
	data := []byte(`{"id":"42","name":"Ada","active":true}`)

	valueCache := &Cache[benchmarkCacheUser]{codec: codec}
	pointerCache := &Cache[*benchmarkCacheUser]{codec: codec}

	b.Run("value/direct", func(b *testing.B) {
		b.ReportAllocs()

		b.RunParallel(func(pb *testing.PB) {
			var value benchmarkCacheUser

			for pb.Next() {
				decoded, err := decodeCacheUserValue(codec, data)
				if err != nil {
					panic(err)
				}

				value = decoded
			}

			runtime.KeepAlive(value)
		})
	})

	b.Run("value/legacy", func(b *testing.B) {
		b.ReportAllocs()

		b.RunParallel(func(pb *testing.PB) {
			var value benchmarkCacheUser

			for pb.Next() {
				decoded, err := decodeCacheValueLegacy[benchmarkCacheUser](
					codec,
					data,
				)
				if err != nil {
					panic(err)
				}

				value = decoded
			}

			runtime.KeepAlive(value)
		})
	})

	b.Run("value/current", func(b *testing.B) {
		b.ReportAllocs()

		b.RunParallel(func(pb *testing.PB) {
			var value benchmarkCacheUser

			for pb.Next() {
				decoded, err := valueCache.decode(nil, data)
				if err != nil {
					panic(err)
				}

				value = decoded
			}

			runtime.KeepAlive(value)
		})
	})

	b.Run("pointer/direct", func(b *testing.B) {
		b.ReportAllocs()

		b.RunParallel(func(pb *testing.PB) {
			var value *benchmarkCacheUser

			for pb.Next() {
				decoded, err := decodeCacheUserPointer(codec, data)
				if err != nil {
					panic(err)
				}

				value = decoded
			}

			runtime.KeepAlive(value)
		})
	})

	b.Run("pointer/legacy", func(b *testing.B) {
		b.ReportAllocs()

		b.RunParallel(func(pb *testing.PB) {
			var value *benchmarkCacheUser

			for pb.Next() {
				decoded, err := decodeCacheValueLegacy[*benchmarkCacheUser](
					codec,
					data,
				)
				if err != nil {
					panic(err)
				}

				value = decoded
			}

			runtime.KeepAlive(value)
		})
	})

	b.Run("pointer/current", func(b *testing.B) {
		b.ReportAllocs()

		b.RunParallel(func(pb *testing.PB) {
			var value *benchmarkCacheUser

			for pb.Next() {
				decoded, err := pointerCache.decode(nil, data)
				if err != nil {
					panic(err)
				}

				value = decoded
			}

			runtime.KeepAlive(value)
		})
	})
}
