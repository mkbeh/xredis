package xredis

import (
	"context"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	rdb "github.com/redis/go-redis/v9"
)

// rateLimitFixedWindowScript atomically checks and updates a fixed-window limit.
//
// KEYS[1] - rate limit key
// ARGV[1] - request limit
// ARGV[2] - window duration in milliseconds
var rateLimitFixedWindowScript = rdb.NewScript(`
local limit = tonumber(ARGV[1])
local window = tonumber(ARGV[2])

local raw = redis.call("GET", KEYS[1])
local current = tonumber(raw)

if raw and not current then
	return redis.error_reply("ERR rate limit key contains non-integer value")
end

if not current then
	current = 0
end

local ttl = redis.call("PTTL", KEYS[1])
if ttl < 0 and current > 0 then
	redis.call("PEXPIRE", KEYS[1], window)
	ttl = window
elseif ttl < 0 then
	ttl = window
end

if current >= limit then
	return {0, limit, 0, ttl, ttl}
end

current = redis.call("INCR", KEYS[1])

if current == 1 then
	redis.call("PEXPIRE", KEYS[1], window)
	ttl = window
else
	ttl = redis.call("PTTL", KEYS[1])
	if ttl < 0 then
		redis.call("PEXPIRE", KEYS[1], window)
		ttl = window
	end
end

local remaining = limit - current
if remaining < 0 then
	remaining = 0
end

return {1, limit, remaining, 0, ttl}
`)

// rateLimitSlidingWindowScript atomically checks and updates a sliding-window limit.
//
// KEYS[1] - rate limit key
// ARGV[1] - request limit
// ARGV[2] - window duration in milliseconds
// ARGV[3] - unique request member
var rateLimitSlidingWindowScript = rdb.NewScript(`
local limit = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local member = ARGV[3]

local time = redis.call("TIME")
local now = tonumber(time[1]) * 1000 + math.floor(tonumber(time[2]) / 1000)
local min = now - window

redis.call("ZREMRANGEBYSCORE", KEYS[1], "-inf", min)

local current = redis.call("ZCARD", KEYS[1])
local allowed = 0

if current > 0 then
	local ttl = redis.call("PTTL", KEYS[1])
	if ttl < 0 then
		redis.call("PEXPIRE", KEYS[1], window)
	end
end

if current < limit then
	allowed = 1
	redis.call("ZADD", KEYS[1], now, member)
	redis.call("PEXPIRE", KEYS[1], window)
	current = current + 1
end

local remaining = limit - current
if remaining < 0 then
	remaining = 0
end

local retry_after = 0
local reset_after = 0
local oldest = redis.call("ZRANGE", KEYS[1], 0, 0, "WITHSCORES")

if oldest[2] then
	reset_after = tonumber(oldest[2]) + window - now
	if reset_after < 0 then
		reset_after = 0
	end
end

if allowed == 0 then
	retry_after = reset_after
end

return {allowed, limit, remaining, retry_after, reset_after}
`)

// rateLimitTokenBucketScript atomically checks and updates a token-bucket limit.
//
// KEYS[1] - rate limit key
// ARGV[1] - bucket capacity
// ARGV[2] - refill tokens per window
// ARGV[3] - refill window in milliseconds
var rateLimitTokenBucketScript = rdb.NewScript(`
local capacity = tonumber(ARGV[1])
local refill = tonumber(ARGV[2])
local window = tonumber(ARGV[3])

local time = redis.call("TIME")
local now = tonumber(time[1]) * 1000 + math.floor(tonumber(time[2]) / 1000)

local data = redis.call("HMGET", KEYS[1], "tokens", "updated_at")
local tokens = tonumber(data[1])
local updated_at = tonumber(data[2])

if not tokens then
	tokens = capacity
end

if not updated_at then
	updated_at = now
end

if updated_at > now then
	updated_at = now
end

if now > updated_at then
	local refill_tokens = (now - updated_at) * refill / window
	tokens = math.min(capacity, tokens + refill_tokens)
	updated_at = now
end

local allowed = 0
if tokens >= 1 then
	allowed = 1
	tokens = tokens - 1
end

local retry_after = 0
if allowed == 0 then
	retry_after = math.ceil((1 - tokens) * window / refill)
end

local reset_after = 0
if tokens < capacity then
	reset_after = math.ceil((capacity - tokens) * window / refill)
end

redis.call("HSET", KEYS[1], "tokens", tostring(tokens), "updated_at", updated_at)

local ttl = reset_after
if ttl < window then
	ttl = window
end

redis.call("PEXPIRE", KEYS[1], ttl)

return {allowed, capacity, math.floor(tokens), retry_after, reset_after}
`)

// RateLimiter applies Redis-backed application rate limits.
type RateLimiter struct {
	client *Client
	prefix string

	id  string
	seq atomic.Uint64
}

// RateLimiterOption configures RateLimiter.
type RateLimiterOption func(*rateLimiterOptions)

type rateLimiterOptions struct {
	prefix string
}

// RateLimit configures fixed-window and sliding-window rate limits.
type RateLimit struct {
	// Limit defines how many requests are allowed within the window.
	Limit int64

	// Window defines the rate limit window duration.
	Window time.Duration
}

// TokenBucketRateLimit configures token-bucket rate limits.
type TokenBucketRateLimit struct {
	// Limit defines how many tokens are refilled per window.
	Limit int64

	// Window defines the refill interval for Limit tokens.
	Window time.Duration

	// Burst defines the maximum number of tokens stored in the bucket.
	// If Burst is zero, Limit is used as bucket capacity.
	Burst int64
}

// RateLimitDecision describes the result of a rate limit check.
type RateLimitDecision struct {
	// Allowed reports whether the request is allowed.
	Allowed bool

	// Limit is the effective limit for the selected algorithm.
	//
	// For fixed-window and sliding-window limits, it equals RateLimit.Limit.
	// For token-bucket limits, it equals bucket capacity.
	Limit int64

	// Remaining is the number of requests or tokens left after this decision.
	Remaining int64

	// RetryAfter tells how long the caller should wait before retrying.
	// It is zero when the request is allowed.
	RetryAfter time.Duration

	// ResetAfter tells how long it takes until the current limit state resets
	// or the bucket becomes full again.
	ResetAfter time.Duration
}

// NewRateLimiter creates a Redis-backed application rate limiter.
func NewRateLimiter(client *Client, opts ...RateLimiterOption) (*RateLimiter, error) {
	return newRateLimiter(client, opts...)
}

// RateLimiter creates a Redis-backed application rate limiter bound to this client.
func (c *Client) RateLimiter(opts ...RateLimiterOption) (*RateLimiter, error) {
	return newRateLimiter(c, opts...)
}

func newRateLimiter(client *Client, opts ...RateLimiterOption) (*RateLimiter, error) {
	if client == nil || client.conn == nil {
		return nil, ErrInvalidRateLimiter
	}

	var options rateLimiterOptions
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}

	return &RateLimiter{
		client: client,
		prefix: options.prefix,
		id:     uuid.NewString(),
	}, nil
}

// WithRateLimiterPrefix configures key prefix for rate limit keys.
func WithRateLimiterPrefix(prefix string) RateLimiterOption {
	return func(opts *rateLimiterOptions) {
		opts.prefix = prefix
	}
}

// Allow checks a fixed-window rate limit.
//
// It is equivalent to AllowFixedWindow.
func (l *RateLimiter) Allow(ctx context.Context, key string, limit RateLimit) (RateLimitDecision, error) {
	return l.AllowFixedWindow(ctx, key, limit)
}

// AllowFixedWindow checks a fixed-window rate limit.
//
// Fixed window is simple and cheap, but can allow bursts around window boundaries.
func (l *RateLimiter) AllowFixedWindow(
	ctx context.Context,
	key string,
	limit RateLimit,
) (RateLimitDecision, error) {
	if err := l.validateKey(key); err != nil {
		return RateLimitDecision{}, err
	}

	if err := validateRateLimit(limit); err != nil {
		return RateLimitDecision{}, err
	}

	result, err := rateLimitFixedWindowScript.Run(
		ctx,
		l.client.conn,
		[]string{l.key(key)},
		limit.Limit,
		durationToMs(limit.Window),
	).Slice()
	if err != nil {
		return RateLimitDecision{}, err
	}

	return parseRateLimitDecision(result)
}

// AllowSlidingWindow checks a sliding-window rate limit.
//
// Sliding window is more accurate than fixed window, but it stores one sorted-set
// entry per accepted request within the window.
func (l *RateLimiter) AllowSlidingWindow(
	ctx context.Context,
	key string,
	limit RateLimit,
) (RateLimitDecision, error) {
	if err := l.validateKey(key); err != nil {
		return RateLimitDecision{}, err
	}

	if err := validateRateLimit(limit); err != nil {
		return RateLimitDecision{}, err
	}

	result, err := rateLimitSlidingWindowScript.Run(
		ctx,
		l.client.conn,
		[]string{l.key(key)},
		limit.Limit,
		durationToMs(limit.Window),
		l.nextMember(),
	).Slice()
	if err != nil {
		return RateLimitDecision{}, err
	}

	return parseRateLimitDecision(result)
}

// AllowTokenBucket checks a token-bucket rate limit.
//
// Token bucket allows controlled bursts up to Burst while refilling Limit tokens
// per Window.
func (l *RateLimiter) AllowTokenBucket(
	ctx context.Context,
	key string,
	limit TokenBucketRateLimit,
) (RateLimitDecision, error) {
	if err := l.validateKey(key); err != nil {
		return RateLimitDecision{}, err
	}

	burst, err := validateTokenBucketRateLimit(limit)
	if err != nil {
		return RateLimitDecision{}, err
	}

	result, err := rateLimitTokenBucketScript.Run(
		ctx,
		l.client.conn,
		[]string{l.key(key)},
		burst,
		limit.Limit,
		durationToMs(limit.Window),
	).Slice()
	if err != nil {
		return RateLimitDecision{}, err
	}

	return parseRateLimitDecision(result)
}

func (l *RateLimiter) validateKey(key string) error {
	if l == nil || l.client == nil || l.client.conn == nil {
		return ErrInvalidRateLimiter
	}

	if key == "" {
		return ErrInvalidRateLimit
	}

	return nil
}

func (l *RateLimiter) key(key string) string {
	return l.prefix + key
}

func (l *RateLimiter) nextMember() string {
	return l.id + ":" + strconv.FormatUint(l.seq.Add(1), 10)
}

func validateRateLimit(limit RateLimit) error {
	if limit.Limit <= 0 || limit.Window <= 0 {
		return ErrInvalidRateLimit
	}

	return nil
}

func validateTokenBucketRateLimit(limit TokenBucketRateLimit) (int64, error) {
	if limit.Limit <= 0 || limit.Window <= 0 || limit.Burst < 0 {
		return 0, ErrInvalidRateLimit
	}

	burst := limit.Burst
	if burst == 0 {
		burst = limit.Limit
	}

	return burst, nil
}

func parseRateLimitDecision(result []any) (RateLimitDecision, error) {
	if len(result) != 5 {
		return RateLimitDecision{}, ErrInvalidRateLimit
	}

	allowed, err := rateLimitResultInt(result[0])
	if err != nil {
		return RateLimitDecision{}, err
	}

	limit, err := rateLimitResultInt(result[1])
	if err != nil {
		return RateLimitDecision{}, err
	}

	remaining, err := rateLimitResultInt(result[2])
	if err != nil {
		return RateLimitDecision{}, err
	}

	retryAfterMs, err := rateLimitResultInt(result[3])
	if err != nil {
		return RateLimitDecision{}, err
	}

	resetAfterMs, err := rateLimitResultInt(result[4])
	if err != nil {
		return RateLimitDecision{}, err
	}

	return RateLimitDecision{
		Allowed:    allowed == 1,
		Limit:      limit,
		Remaining:  remaining,
		RetryAfter: msToDuration(retryAfterMs),
		ResetAfter: msToDuration(resetAfterMs),
	}, nil
}

func rateLimitResultInt(value any) (int64, error) {
	v, ok := value.(int64)
	if !ok {
		return 0, ErrInvalidRateLimit
	}

	return v, nil
}
