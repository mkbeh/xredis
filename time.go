package xredis

import "time"

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
