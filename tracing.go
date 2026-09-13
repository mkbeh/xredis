package xredis

// Tracing instruments a Client with distributed tracing.
//
// Implementations must be safe to reuse across multiple clients. Instrument may
// be called concurrently.
type Tracing interface {
	Instrument(client *Client) error
}
