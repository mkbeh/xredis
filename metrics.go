package xredis

import (
	"context"
	"time"
)

// Metrics provides wrapper-level metrics instrumentation for a Client.
//
// Implementations must be safe to reuse across multiple clients. Register may
// be called concurrently. Metrics returned for a client must be safe for
// concurrent use.
type Metrics interface {
	Register(client *Client) ClientMetrics
}

// ClientMetrics contains domain-specific metrics bound to a Client.
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
	cache   cacheMetrics
	lock    lockMetrics
	limiter rateLimiterMetrics
}

func newClientMetrics(metrics ClientMetrics) clientMetrics {
	return clientMetrics{
		cache: cacheMetrics{
			metrics: metrics.Cache,
		},
		lock: lockMetrics{
			metrics: metrics.Lock,
		},
		limiter: rateLimiterMetrics{
			metrics: metrics.RateLimiter,
		},
	}
}

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

type lockMetrics struct {
	metrics LockMetrics
}

func (m lockMetrics) recordOperation(ctx context.Context, lockType, operation, outcome string) {
	if m.metrics == nil {
		return
	}

	m.metrics.RecordOperation(ctx, lockType, operation, outcome)
}

type rateLimiterMetrics struct {
	metrics RateLimiterMetrics
}

func (m rateLimiterMetrics) recordDecision(ctx context.Context, algorithm, outcome string, duration time.Duration) {
	if m.metrics == nil {
		return
	}

	m.metrics.RecordDecision(ctx, algorithm, outcome, duration)
}

const (
	cacheOperationGet       = "get"
	cacheOperationGetOrLoad = "get_or_load"
)

const (
	cacheResultHit         = "hit"
	cacheResultMiss        = "miss"
	cacheResultNegativeHit = "negative_hit"
	cacheResultError       = "error"
)

const (
	loaderOutcomeSuccess  = "success"
	loaderOutcomeNotFound = "not_found"
	loaderOutcomeError    = "error"
)

const (
	lockTypeLease  = "lease"
	lockTypeFenced = "fenced"
)

const (
	lockOperationAcquire = "acquire"
	lockOperationExtend  = "extend"
	lockOperationUnlock  = "unlock"
)

const (
	lockOutcomeSuccess   = "success"
	lockOutcomeContended = "contended"
	lockOutcomeNotOwned  = "not_owned"
	lockOutcomeError     = "error"
)

const (
	rateLimitAlgorithmFixedWindow   = "fixed_window"
	rateLimitAlgorithmSlidingWindow = "sliding_window"
	rateLimitAlgorithmTokenBucket   = "token_bucket"
)

const (
	rateLimitOutcomeAllowed  = "allowed"
	rateLimitOutcomeRejected = "rejected"
	rateLimitOutcomeError    = "error"
)
