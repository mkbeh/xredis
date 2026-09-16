package otelxredis

import (
	"slices"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

func TestMetricsRegister(t *testing.T) {
	t.Parallel()

	metrics, _ := newTestMetrics(t)
	registered := metrics.Register()

	if registered.Cache == nil {
		t.Fatal("cache metrics were not registered")
	}
	if registered.Lock == nil {
		t.Fatal("lock metrics were not registered")
	}
	if registered.RateLimiter == nil {
		t.Fatal("rate limiter metrics were not registered")
	}
}

func TestCacheMetrics(t *testing.T) {
	t.Parallel()

	metrics, reader := newTestMetrics(
		t,
		WithClientID("client-a"),
		WithLabel("service", "orders"),
	)

	cache := metrics.Register().Cache
	cache.RecordRequest(t.Context(), "get", "hit")
	cache.RecordLoaderDuration(t.Context(), "success", 250*time.Millisecond)
	cache.RecordSingleflightShared(t.Context())

	collected := collectMetrics(t, reader)

	requests := onlySumPoint(t, sumData(t, collected["xredis.cache.requests"]))
	if requests.Value != 1 {
		t.Fatalf("cache requests = %d, want 1", requests.Value)
	}
	assertAttributes(
		t,
		requests.Attributes.ToSlice(),
		attribute.String("xredis.client.id", "client-a"),
		attribute.String("service", "orders"),
		attribute.String("xredis.cache.operation", "get"),
		attribute.String("xredis.cache.result", "hit"),
	)

	loader := onlyHistogramPoint(
		t,
		durationHistogram(t, collected["xredis.cache.loader.duration"]),
	)
	assertHistogramPoint(t, loader, 0.25)
	assertAttributes(
		t,
		loader.Attributes.ToSlice(),
		attribute.String("xredis.client.id", "client-a"),
		attribute.String("service", "orders"),
		attribute.String("xredis.cache.loader.outcome", "success"),
	)

	shared := onlySumPoint(t, sumData(t, collected["xredis.cache.singleflight.shared"]))
	if shared.Value != 1 {
		t.Fatalf("singleflight shared requests = %d, want 1", shared.Value)
	}
	assertAttributes(
		t,
		shared.Attributes.ToSlice(),
		attribute.String("xredis.client.id", "client-a"),
		attribute.String("service", "orders"),
	)
}

func TestLockMetrics(t *testing.T) {
	t.Parallel()

	metrics, reader := newTestMetrics(
		t,
		WithClientID("client-a"),
		WithLabel("service", "orders"),
	)

	metrics.Register().Lock.RecordOperation(
		t.Context(),
		"lease",
		"acquire",
		"success",
	)

	point := onlySumPoint(
		t,
		sumData(t, collectMetrics(t, reader)["xredis.lock.operations"]),
	)
	if point.Value != 1 {
		t.Fatalf("lock operations = %d, want 1", point.Value)
	}
	assertAttributes(
		t,
		point.Attributes.ToSlice(),
		attribute.String("xredis.client.id", "client-a"),
		attribute.String("service", "orders"),
		attribute.String("xredis.lock.type", "lease"),
		attribute.String("xredis.lock.operation", "acquire"),
		attribute.String("xredis.lock.outcome", "success"),
	)
}

func TestRateLimiterMetrics(t *testing.T) {
	t.Parallel()

	metrics, reader := newTestMetrics(
		t,
		WithClientID("client-a"),
		WithLabel("service", "orders"),
	)

	metrics.Register().RateLimiter.RecordDecision(
		t.Context(),
		"fixed_window",
		"allowed",
		10*time.Millisecond,
	)

	collected := collectMetrics(t, reader)

	decision := onlySumPoint(t, sumData(t, collected["xredis.rate_limiter.decisions"]))
	if decision.Value != 1 {
		t.Fatalf("rate limiter decisions = %d, want 1", decision.Value)
	}
	assertAttributes(
		t,
		decision.Attributes.ToSlice(),
		attribute.String("xredis.client.id", "client-a"),
		attribute.String("service", "orders"),
		attribute.String("xredis.rate_limiter.algorithm", "fixed_window"),
		attribute.String("xredis.rate_limiter.outcome", "allowed"),
	)

	duration := onlyHistogramPoint(
		t,
		durationHistogram(t, collected["xredis.rate_limiter.duration"]),
	)
	assertHistogramPoint(t, duration, 0.01)
	assertAttributes(
		t,
		duration.Attributes.ToSlice(),
		attribute.String("xredis.client.id", "client-a"),
		attribute.String("service", "orders"),
		attribute.String("xredis.rate_limiter.algorithm", "fixed_window"),
		attribute.String("xredis.rate_limiter.outcome", "allowed"),
	)
}

func TestMetricsDurationBuckets(t *testing.T) {
	t.Parallel()

	metrics, reader := newTestMetrics(t)
	registered := metrics.Register()
	registered.Cache.RecordLoaderDuration(t.Context(), "success", time.Millisecond)
	registered.RateLimiter.RecordDecision(t.Context(), "fixed_window", "allowed", time.Millisecond)

	collected := collectMetrics(t, reader)

	cache := onlyHistogramPoint(
		t,
		durationHistogram(t, collected["xredis.cache.loader.duration"]),
	)
	wantCache := []float64{
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
	if !slices.Equal(cache.Bounds, wantCache) {
		t.Fatalf("cache loader duration bounds = %v, want %v", cache.Bounds, wantCache)
	}

	rateLimiter := onlyHistogramPoint(
		t,
		durationHistogram(t, collected["xredis.rate_limiter.duration"]),
	)
	wantRateLimiter := []float64{
		0.001,
		0.005,
		0.01,
		0.05,
		0.1,
		0.5,
		1,
		5,
		10,
	}
	if !slices.Equal(rateLimiter.Bounds, wantRateLimiter) {
		t.Fatalf("rate limiter duration bounds = %v, want %v", rateLimiter.Bounds, wantRateLimiter)
	}
}
