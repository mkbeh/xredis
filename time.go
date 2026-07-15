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

func expirationToMs(expiration time.Duration) int64 {
	if expiration == KeepTTL {
		return -1
	}

	return durationToMs(expiration)
}

func validateCreateExpiration(expiration time.Duration) error {
	if expiration < 0 {
		return ErrInvalidTTL
	}

	return nil
}

func validateUpdateExpiration(expiration time.Duration) error {
	if expiration == KeepTTL || expiration >= 0 {
		return nil
	}

	return ErrInvalidTTL
}
