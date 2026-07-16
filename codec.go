package xredis

import "encoding/json"

// Codec encodes and decodes values stored in Redis.
//
// Implementations must be safe for concurrent use.
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
