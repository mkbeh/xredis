package main

import (
	"context"
	"errors"
	"net/http"

	"github.com/mkbeh/xredis"
	promclient "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

type metricsRuntime struct {
	registry      *promclient.Registry
	meterProvider *sdkmetric.MeterProvider
	shutdownRedis func() error
}

func newMetricsRuntime() (*metricsRuntime, error) {
	registry := promclient.NewRegistry()

	exporter, err := otelprom.New(
		otelprom.WithRegisterer(registry),
	)
	if err != nil {
		return nil, err
	}

	res, err := resource.New(
		context.Background(),
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
		resource.WithAttributes(
			semconv.ServiceName("xredis-locks-example"),
			semconv.ServiceVersion("dev"),
		),
	)
	if err != nil {
		return nil, err
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(exporter),
	)

	// Wrapper-level metrics will use the same provider later.
	otel.SetMeterProvider(meterProvider)

	shutdownRedis, err := xredis.InitObservability(
		xredis.WithMeterProvider(meterProvider),
		xredis.WithRedisMetricGroups(xredis.RedisMetricGroupDefault),
	)
	if err != nil {
		_ = meterProvider.Shutdown(context.Background())
		return nil, err
	}

	return &metricsRuntime{
		registry:      registry,
		meterProvider: meterProvider,
		shutdownRedis: shutdownRedis,
	}, nil
}

func (m *metricsRuntime) Handler() http.Handler {
	return promhttp.HandlerFor(
		m.registry,
		promhttp.HandlerOpts{},
	)
}

func (m *metricsRuntime) Shutdown(ctx context.Context) error {
	return errors.Join(
		m.shutdownRedis(),
		m.meterProvider.Shutdown(ctx),
	)
}
