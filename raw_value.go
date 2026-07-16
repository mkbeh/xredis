package xredis

import (
	"net"
	"time"
)

func isRawValueType[T any]() bool {
	var value T

	switch any(value).(type) {
	case string, *string,
		[]byte,
		int, *int,
		int8, *int8,
		int16, *int16,
		int32, *int32,
		int64, *int64,
		uint, *uint,
		uint8, *uint8,
		uint16, *uint16,
		uint32, *uint32,
		uint64, *uint64,
		float32, *float32,
		float64, *float64,
		bool, *bool,
		time.Time, *time.Time,
		time.Duration, *time.Duration,
		net.IP:
		return true
	default:
		return false
	}
}
