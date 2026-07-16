package xredis

import (
	"time"

	rdb "github.com/redis/go-redis/v9"
)

// KeepTTL preserves the existing key expiration during an update.
const KeepTTL time.Duration = rdb.KeepTTL

func durationToMs(d time.Duration) int64 {
	if d <= 0 {
		return int64(d / time.Millisecond)
	}

	ms := d / time.Millisecond
	if d%time.Millisecond != 0 {
		ms++
	}

	return int64(ms)
}

func msToDuration(ms int64) time.Duration {
	if ms <= 0 {
		return 0
	}

	return time.Duration(ms) * time.Millisecond
}

func updateExpirationToMs(expiration time.Duration) (int64, error) {
	switch {
	case expiration == KeepTTL:
		return -1, nil
	case expiration >= 0:
		return durationToMs(expiration), nil
	default:
		return 0, ErrInvalidTTL
	}
}
