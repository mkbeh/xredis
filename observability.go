package xredis

import (
	"context"
	"time"
)

// Metrics provides wrapper-level metrics instrumentation.
//
// Implementations must be safe to reuse across multiple clients. Register may
// be called concurrently. ClientMetrics returned by Register must be safe for
// concurrent use.
type Metrics interface {
	Register() ClientMetrics
}

// Tracing instruments a Client with distributed tracing.
//
// Implementations must be safe to reuse across multiple clients. Instrument may
// be called concurrently.
type Tracing interface {
	Instrument(client *Client) error
}

// ClientMetrics contains domain-specific wrapper metrics.
type ClientMetrics struct {
	Cache       CacheMetrics
	Lock        LockMetrics
	RateLimiter RateLimiterMetrics
}

// CacheMetrics records cache metrics.
type CacheMetrics interface {
	RecordRequest(ctx context.Context, operation, result string)
	RecordLoaderDuration(ctx context.Context, outcome string, duration time.Duration)
	RecordSingleflightShared(ctx context.Context)
}

// LockMetrics records distributed lock metrics.
type LockMetrics interface {
	RecordOperation(ctx context.Context, lockType, operation, outcome string)
}

// RateLimiterMetrics records rate limiter metrics.
type RateLimiterMetrics interface {
	RecordDecision(ctx context.Context, algorithm, outcome string, duration time.Duration)
}

type clientMetrics struct {
	cache       cacheMetrics
	lock        lockMetrics
	rateLimiter rateLimiterMetrics
}

func newClientMetrics(metrics ClientMetrics) clientMetrics {
	return clientMetrics{
		cache: cacheMetrics{
			metrics: metrics.Cache,
		},
		lock: lockMetrics{
			metrics: metrics.Lock,
		},
		rateLimiter: rateLimiterMetrics{
			metrics: metrics.RateLimiter,
		},
	}
}

const (
	cacheOperationGet       = "get"
	cacheOperationGetOrLoad = "get_or_load"

	cacheResultHit         = "hit"
	cacheResultMiss        = "miss"
	cacheResultNegativeHit = "negative_hit"
	cacheResultError       = "error"

	cacheLoaderOutcomeSuccess  = "success"
	cacheLoaderOutcomeNotFound = "not_found"
	cacheLoaderOutcomeError    = "error"
)

type cacheMetrics struct {
	metrics CacheMetrics
}

func (m cacheMetrics) recordRequest(ctx context.Context, operation, result string) {
	if m.metrics == nil {
		return
	}

	m.metrics.RecordRequest(ctx, operation, result)
}

func (m cacheMetrics) recordLoaderDuration(ctx context.Context, outcome string, duration time.Duration) {
	if m.metrics == nil {
		return
	}

	m.metrics.RecordLoaderDuration(ctx, outcome, duration)
}

func (m cacheMetrics) recordSingleflightShared(ctx context.Context) {
	if m.metrics == nil {
		return
	}

	m.metrics.RecordSingleflightShared(ctx)
}

const (
	lockTypeLease  = "lease"
	lockTypeFenced = "fenced"

	lockOperationAcquire = "acquire"
	lockOperationExtend  = "extend"
	lockOperationUnlock  = "unlock"

	lockOutcomeSuccess   = "success"
	lockOutcomeContended = "contended"
	lockOutcomeNotOwned  = "not_owned"
	lockOutcomeError     = "error"
)

type lockMetrics struct {
	metrics LockMetrics
}

func (m lockMetrics) recordOperation(ctx context.Context, lockType, operation, outcome string) {
	if m.metrics == nil {
		return
	}

	m.metrics.RecordOperation(ctx, lockType, operation, outcome)
}

const (
	rateLimiterAlgorithmFixedWindow   = "fixed_window"
	rateLimiterAlgorithmSlidingWindow = "sliding_window"
	rateLimiterAlgorithmTokenBucket   = "token_bucket"

	rateLimiterOutcomeAllowed  = "allowed"
	rateLimiterOutcomeRejected = "rejected"
	rateLimiterOutcomeError    = "error"
)

type rateLimiterMetrics struct {
	metrics RateLimiterMetrics
}

func (m rateLimiterMetrics) recordDecision(ctx context.Context, algorithm, outcome string, duration time.Duration) {
	if m.metrics == nil {
		return
	}

	m.metrics.RecordDecision(ctx, algorithm, outcome, duration)
}
