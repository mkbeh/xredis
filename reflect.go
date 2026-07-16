package xredis

import (
	"fmt"
	"reflect"
)

func validateConcreteType[T any]() error {
	typ := reflect.TypeFor[T]()

	if typ.Kind() == reflect.Interface {
		return fmt.Errorf("%w: interface value type %s is not supported", ErrUnsupportedType, typ)
	}

	return nil
}

// decodeInto decodes a value of type T by passing an addressable destination
// to decode.
//
// For a non-pointer T, decode receives *T. For a pointer T, decodeInto
// allocates the pointed-to value and passes the resulting pointer to decode.
// Defined pointer types are converted back to T after decoding.
//
// ErrInvalidEntry is returned if the decoded pointer cannot be represented as T.
func decodeInto[T any](decode func(dst any) error) (T, error) {
	var zero T

	typ := reflect.TypeFor[T]()

	if typ.Kind() != reflect.Pointer {
		var value T

		if err := decode(&value); err != nil {
			return zero, err
		}

		return value, nil
	}

	value := reflect.New(typ.Elem())

	if err := decode(value.Interface()); err != nil {
		return zero, err
	}

	// reflect.New returns PointerTo(typ.Elem()), which may differ from T when
	// T is a defined pointer type.
	if value.Type() != typ {
		if !value.CanConvert(typ) {
			return zero, ErrInvalidEntry
		}

		value = value.Convert(typ)
	}

	decoded, ok := value.Interface().(T)
	if !ok {
		return zero, ErrInvalidEntry
	}

	return decoded, nil
}
