package xredis

import "encoding/json"

// Codec encodes and decodes values stored in Redis string values.
//
// Implementations must be safe for concurrent use by multiple goroutines.
//
// The byte slice returned by Marshal must remain valid and unchanged after
// Marshal returns.
//
// Unmarshal must treat data as read-only and must not retain it after returning
// unless it makes a copy.
type Codec interface {
	Marshal(value any) ([]byte, error)
	Unmarshal(data []byte, value any) error
}

// JSONCodec encodes and decodes values using encoding/json.
type JSONCodec struct{}

func (JSONCodec) Marshal(value any) ([]byte, error) {
	return json.Marshal(value)
}

func (JSONCodec) Unmarshal(data []byte, value any) error {
	return json.Unmarshal(data, value)
}
