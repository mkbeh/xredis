package xredis

import (
	"context"
	"testing"
	"time"
)

type testMetrics struct {
	client  *Client
	metrics ClientMetrics
}

func (m *testMetrics) Register(client *Client) ClientMetrics {
	m.client = client

	return m.metrics
}

type testClientMetrics struct{}

func (*testClientMetrics) RecordRequest(context.Context, string, string) {}

func (*testClientMetrics) RecordLoaderDuration(
	context.Context,
	string,
	time.Duration,
) {
}

func (*testClientMetrics) RecordSingleflightShared(context.Context) {}

func (*testClientMetrics) RecordOperation(
	context.Context,
	string,
	string,
	string,
) {
}

func (*testClientMetrics) RecordDecision(
	context.Context,
	string,
	string,
	time.Duration,
) {
}

type testTracing struct {
	client *Client
}

func (t *testTracing) Instrument(client *Client) error {
	t.client = client

	return nil
}

func TestClientObservability(t *testing.T) {
	clientMetrics := &testClientMetrics{}
	metrics := &testMetrics{
		metrics: ClientMetrics{
			Cache:       clientMetrics,
			Lock:        clientMetrics,
			RateLimiter: clientMetrics,
		},
	}
	tracing := &testTracing{}

	client, err := NewClient(
		WithMetrics(metrics),
		WithTracing(tracing),
		WithMetricLabel("service", "orders"),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := client.Close(); closeErr != nil {
			t.Fatal(closeErr)
		}
	}()

	if metrics.client != client {
		t.Fatal("metrics registered with unexpected client")
	}

	if tracing.client != client {
		t.Fatal("tracing instrumented unexpected client")
	}

	if client.metrics.cache.metrics != clientMetrics {
		t.Fatal("cache metrics were not attached to client")
	}

	if client.metrics.lock.metrics != clientMetrics {
		t.Fatal("lock metrics were not attached to client")
	}

	if client.metrics.limiter.metrics != clientMetrics {
		t.Fatal("rate limiter metrics were not attached to client")
	}

	labels := client.Labels()
	if labels["service"] != "orders" {
		t.Fatalf("unexpected service label: %q", labels["service"])
	}

	labels["service"] = "changed"
	if client.Labels()["service"] != "orders" {
		t.Fatal("client labels must be returned as a copy")
	}
}
