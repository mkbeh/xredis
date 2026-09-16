package otelxredis

import (
	"context"
	"math"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

const testScopeName = "github.com/mkbeh/xredis/extra/otelxredis"

func newTestMetrics(t *testing.T, opts ...MetricsOption) (*Metrics, *sdkmetric.ManualReader) {
	t.Helper()

	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() {
		if err := provider.Shutdown(context.WithoutCancel(t.Context())); err != nil {
			t.Error(err)
		}
	})

	opts = append(opts, WithMeterProvider(provider))
	metrics, err := NewMetrics(opts...)
	if err != nil {
		t.Fatal(err)
	}

	return metrics, reader
}

func collectScopeMetrics(t *testing.T, reader *sdkmetric.ManualReader) metricdata.ScopeMetrics {
	t.Helper()

	var data metricdata.ResourceMetrics
	if err := reader.Collect(t.Context(), &data); err != nil {
		t.Fatal(err)
	}
	if len(data.ScopeMetrics) != 1 {
		t.Fatalf("scope metrics = %d, want 1", len(data.ScopeMetrics))
	}

	scope := data.ScopeMetrics[0]
	if scope.Scope.Name != testScopeName {
		t.Fatalf("scope name = %q, want %q", scope.Scope.Name, testScopeName)
	}

	return scope
}

func collectMetrics(t *testing.T, reader *sdkmetric.ManualReader) map[string]metricdata.Metrics {
	t.Helper()

	scope := collectScopeMetrics(t, reader)
	metrics := make(map[string]metricdata.Metrics, len(scope.Metrics))
	for _, instrument := range scope.Metrics {
		if _, ok := metrics[instrument.Name]; ok {
			t.Fatalf("duplicate metric %q", instrument.Name)
		}

		metrics[instrument.Name] = instrument
	}

	return metrics
}

func sumData(t *testing.T, data metricdata.Metrics) metricdata.Sum[int64] {
	t.Helper()

	sum, ok := data.Data.(metricdata.Sum[int64])
	if !ok {
		t.Fatalf("metric %q: got %T, want Sum[int64]", data.Name, data.Data)
	}

	return sum
}

func onlySumPoint(t *testing.T, sum metricdata.Sum[int64]) metricdata.DataPoint[int64] {
	t.Helper()

	if len(sum.DataPoints) != 1 {
		t.Fatalf("data points = %d, want 1", len(sum.DataPoints))
	}

	return sum.DataPoints[0]
}

func durationHistogram(t *testing.T, data metricdata.Metrics) metricdata.Histogram[float64] {
	t.Helper()

	histogram, ok := data.Data.(metricdata.Histogram[float64])
	if !ok {
		t.Fatalf("metric %q: got %T, want Histogram[float64]", data.Name, data.Data)
	}
	if data.Unit != "s" {
		t.Fatalf("metric %q unit = %q, want s", data.Name, data.Unit)
	}

	return histogram
}

func onlyHistogramPoint(
	t *testing.T,
	histogram metricdata.Histogram[float64],
) metricdata.HistogramDataPoint[float64] {
	t.Helper()

	if len(histogram.DataPoints) != 1 {
		t.Fatalf("data points = %d, want 1", len(histogram.DataPoints))
	}

	return histogram.DataPoints[0]
}

func assertHistogramPoint(
	t *testing.T,
	point metricdata.HistogramDataPoint[float64],
	sum float64,
) {
	t.Helper()

	if point.Count != 1 {
		t.Fatalf("histogram count = %d, want 1", point.Count)
	}
	if math.Abs(point.Sum-sum) > 1e-12 {
		t.Fatalf("histogram sum = %g, want %g", point.Sum, sum)
	}
}

func assertAttributes(t *testing.T, got []attribute.KeyValue, want ...attribute.KeyValue) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("attributes = %v, want %v", got, want)
	}

	gotSet := attribute.NewSet(got...)
	wantSet := attribute.NewSet(want...)
	if !gotSet.Equals(&wantSet) {
		t.Fatalf("attributes = %v, want %v", gotSet.ToSlice(), wantSet.ToSlice())
	}
}
