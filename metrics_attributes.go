package xredis

const (
	metricAttrCacheOperation = "redis.client.cache.operation"
	metricAttrCacheResult    = "redis.client.cache.result"
	metricAttrLoaderOutcome  = "redis.client.cache.loader.outcome"

	metricAttrLockType      = "redis.client.lock.type"
	metricAttrLockOperation = "redis.client.lock.operation"
	metricAttrLockOutcome   = "redis.client.lock.outcome"

	metricAttrRateLimitAlgorithm = "redis.client.rate_limiter.algorithm"
	metricAttrRateLimitOutcome   = "redis.client.rate_limiter.outcome"
)

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

// Histogram boundaries are expressed in seconds.
var cacheLoaderDurationBuckets = []float64{
	0.005,
	0.01,
	0.025,
	0.05,
	0.075,
	0.1,
	0.25,
	0.5,
	0.75,
	1,
	2.5,
	5,
	7.5,
	10,
}

// Histogram boundaries are expressed in seconds.
var rateLimitDurationBuckets = []float64{
	0.0001,
	0.00025,
	0.0005,
	0.001,
	0.0025,
	0.005,
	0.01,
	0.025,
	0.05,
	0.1,
	0.25,
	0.5,
	1,
}
