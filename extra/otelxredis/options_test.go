package otelxredis

import (
	"maps"
	"sync"
	"testing"

	"go.opentelemetry.io/otel/metric/noop"
)

func TestMetricsOptions(t *testing.T) {
	t.Parallel()

	provider := noop.NewMeterProvider()
	cfg := defaultMetricsConfig()

	opts := []MetricsOption{
		WithMeterProvider(provider),
		WithMeterProvider(nil),
		WithClientID("client-a"),
		WithClientID(""),
		WithLabels(map[string]string{
			"env": "prod",
			"":    "ignored",
		}),
		WithLabel("env", "stage"),
		WithLabel("region", "eu"),
		WithLabel("", "ignored"),
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	if cfg.meterProvider != provider {
		t.Fatal("nil meter provider must preserve the configured provider")
	}
	if cfg.clientID != "client-a" {
		t.Fatalf("client ID = %q, want client-a", cfg.clientID)
	}

	wantLabels := map[string]string{
		"env":    "stage",
		"region": "eu",
	}
	if !maps.Equal(cfg.labels, wantLabels) {
		t.Fatalf("labels = %v, want %v", cfg.labels, wantLabels)
	}
}

func TestWithLabelsSnapshotMergeAndReuse(t *testing.T) {
	t.Parallel()

	input := map[string]string{
		"env":  "prod",
		"team": "a",
	}
	shared := WithLabels(input)

	input["env"] = "changed"
	delete(input, "team")
	input["region"] = "eu"

	first := defaultMetricsConfig()
	shared(&first)
	WithLabels(map[string]string{"team": "b"})(&first)

	wantFirst := map[string]string{
		"env":  "prod",
		"team": "b",
	}
	if !maps.Equal(first.labels, wantFirst) {
		t.Fatalf("merged labels = %v, want %v", first.labels, wantFirst)
	}

	first.labels["env"] = "first only"

	second := defaultMetricsConfig()
	shared(&second)

	wantSecond := map[string]string{
		"env":  "prod",
		"team": "a",
	}
	if !maps.Equal(second.labels, wantSecond) {
		t.Fatalf("reused labels = %v, want %v", second.labels, wantSecond)
	}
}

func TestWithLabelsConcurrentReuse(t *testing.T) {
	t.Parallel()

	shared := WithLabels(map[string]string{
		"env":  "prod",
		"team": "a",
	})

	const goroutines = 16

	var wg sync.WaitGroup

	for range goroutines {
		wg.Go(func() {
			cfg := defaultMetricsConfig()
			shared(&cfg)

			want := map[string]string{
				"env":  "prod",
				"team": "a",
			}
			if !maps.Equal(cfg.labels, want) {
				t.Errorf("labels = %v, want %v", cfg.labels, want)
			}
		})
	}

	wg.Wait()
}
