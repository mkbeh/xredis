package xredis

import (
	"context"

	rdb "github.com/redis/go-redis/v9"
)

// Logger receives internal go-redis log messages.
type Logger interface {
	Printf(ctx context.Context, format string, args ...any)
}

// SetLogger configures the process-wide internal go-redis logger.
//
// Call it during application startup before creating Redis clients.
func SetLogger(logger Logger) {
	if logger != nil {
		rdb.SetLogger(logger)
	}
}
