package xredis

import (
	"context"
	"testing"
	"time"
)

type testMetrics struct {
	metrics ClientMetrics
	calls   int
}

func (m *testMetrics) Register() ClientMetrics {
	m.calls++

	return m.metrics
}

type testCacheMetrics struct{}

func (*testCacheMetrics) RecordRequest(context.Context, string, string) {}

func (*testCacheMetrics) RecordLoaderDuration(context.Context, string, time.Duration) {}

func (*testCacheMetrics) RecordSingleflightShared(context.Context) {}

type testLockMetrics struct{}

func (*testLockMetrics) RecordOperation(context.Context, string, string, string) {}

type testRateLimiterMetrics struct{}

func (*testRateLimiterMetrics) RecordDecision(context.Context, string, string, time.Duration) {}

type testTracing struct {
	client *Client
	calls  int
}

func (t *testTracing) Instrument(client *Client) error {
	t.client = client
	t.calls++

	return nil
}

var (
	_ Metrics            = (*testMetrics)(nil)
	_ CacheMetrics       = (*testCacheMetrics)(nil)
	_ LockMetrics        = (*testLockMetrics)(nil)
	_ RateLimiterMetrics = (*testRateLimiterMetrics)(nil)
	_ Tracing            = (*testTracing)(nil)
)

func TestClientObservability(t *testing.T) {
	cacheMetrics := &testCacheMetrics{}
	lockMetrics := &testLockMetrics{}
	rateLimiterMetrics := &testRateLimiterMetrics{}

	metrics := &testMetrics{
		metrics: ClientMetrics{
			Cache:       cacheMetrics,
			Lock:        lockMetrics,
			RateLimiter: rateLimiterMetrics,
		},
	}
	tracing := &testTracing{}

	client, err := NewClient(
		WithMetrics(metrics),
		WithTracing(tracing),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := client.Close(); closeErr != nil {
			t.Fatal(closeErr)
		}
	})

	if metrics.calls != 1 {
		t.Fatalf("unexpected metrics register calls: %d", metrics.calls)
	}

	if tracing.calls != 1 {
		t.Fatalf("unexpected tracing instrument calls: %d", tracing.calls)
	}

	if tracing.client != client {
		t.Fatal("tracing instrumented unexpected client")
	}

	if client.metrics.cache.metrics != cacheMetrics {
		t.Fatal("cache metrics were not attached to client")
	}

	if client.metrics.lock.metrics != lockMetrics {
		t.Fatal("lock metrics were not attached to client")
	}

	if client.metrics.rateLimiter.metrics != rateLimiterMetrics {
		t.Fatal("rate limiter metrics were not attached to client")
	}
}

func TestClientMetricsPartial(t *testing.T) {
	cacheMetrics := &testCacheMetrics{}

	metrics := newClientMetrics(ClientMetrics{
		Cache: cacheMetrics,
	})

	if metrics.cache.metrics != cacheMetrics {
		t.Fatal("cache metrics were not attached")
	}

	if metrics.lock.metrics != nil {
		t.Fatal("lock metrics must be nil")
	}

	if metrics.rateLimiter.metrics != nil {
		t.Fatal("rate limiter metrics must be nil")
	}

	metrics.lock.recordOperation(t.Context(), "", "", "")
	metrics.rateLimiter.recordDecision(t.Context(), "", "", 0)
}
