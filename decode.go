package xredis

import "reflect"

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
