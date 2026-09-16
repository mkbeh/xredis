package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mkbeh/xredis"
	"github.com/mkbeh/xredis/extra/otelxredis"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	redisotelnative "github.com/redis/go-redis/extra/redisotel-native/v9"
	"github.com/redis/go-redis/extra/redisotel/v9"
	rdb "github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const (
	defaultDB             = 0
	defaultHTTP           = "localhost:8080"
	defaultRedis          = "localhost:6379"
	defaultServiceName    = "xredis-otel-example"
	defaultTracesEndpoint = "http://localhost:4318/v1/traces"
	defaultTTL            = 10 * time.Minute
	shutdownTimeout       = 5 * time.Second

	keyPrefix    = "xredis:otel:"
	sampleClient = "otel-example-client"
)

var (
	client     *xredis.Client
	valueCache *xredis.Cache[string]
	tracer     trace.Tracer
)

type valueRequest struct {
	Value string `json:"value"`
}

func main() {
	redisAddr := env("REDIS_ADDR", defaultRedis)
	httpAddr := env("HTTP_ADDR", defaultHTTP)
	serviceName := env("OTEL_SERVICE_NAME", defaultServiceName)
	tracesEndpoint := env("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", defaultTracesEndpoint)

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	// Create the shared OpenTelemetry resource.
	res, err := newResource(serviceName)
	if err != nil {
		log.Fatalln(err)
	}

	// Create the OpenTelemetry meter provider.
	registry := prometheus.NewRegistry()

	meterProvider, err := newMeterProvider(registry, res)
	if err != nil {
		log.Fatalln(err)
	}
	defer shutdownMeterProvider(meterProvider)

	// Enable native go-redis metrics.
	redisMetrics := redisotelnative.NewConfig().
		WithEnabled(true).
		WithMeterProvider(meterProvider)

	redisObs := redisotelnative.GetObservabilityInstance()
	if err = redisObs.Init(redisMetrics); err != nil {
		log.Fatalln(err)
	}
	defer func() {
		if shutdownErr := redisObs.Shutdown(); shutdownErr != nil {
			log.Printf("shutdown native Redis metrics: %v", shutdownErr)
		}
	}()

	// Create xredis wrapper-level metrics.
	metrics, err := otelxredis.NewMetrics(
		otelxredis.WithMeterProvider(meterProvider),
		otelxredis.WithClientID(sampleClient),
		otelxredis.WithLabel("xredis.example", "otel"),
	)
	if err != nil {
		log.Fatalln(err)
	}

	// Create the OpenTelemetry tracer provider and propagation.
	tracerProvider, err := newTracerProvider(ctx, res, tracesEndpoint)
	if err != nil {
		log.Fatalln(err)
	}
	defer shutdownTracerProvider(tracerProvider)

	propagator := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)

	tracer = tracerProvider.Tracer("github.com/mkbeh/xredis/examples/otel")

	// Create a Redis client with xredis metrics.
	client, err = xredis.NewClient(
		&rdb.Options{
			Addr:       redisAddr,
			DB:         defaultDB,
			ClientName: sampleClient,
			// Keep the example focused on Redis commands rather than maintenance notifications.
			MaintNotificationsConfig: &maintnotifications.Config{
				Mode: maintnotifications.ModeDisabled,
			},
		},
		xredis.WithMetrics(metrics),
	)
	if err != nil {
		log.Fatalln(err)
	}

	// Instrument native Redis command tracing.
	if err = redisotel.InstrumentTracing(
		client.Raw(),
		redisotel.WithTracerProvider(tracerProvider),
		redisotel.WithDBStatement(true),
		redisotel.WithCallerEnabled(true),
		redisotel.WithAttributes(
			attribute.String("xredis.client.id", sampleClient),
			attribute.String("xredis.example", "otel"),
		),
	); err != nil {
		_ = client.Close()
		log.Fatalln(err)
	}
	defer shutdownClient(client)

	// Create a typed cache.
	valueCache, err = client.Cache[string](
		xredis.WithCachePrefix(keyPrefix),
		xredis.WithCacheTTL(defaultTTL),
	)
	if err != nil {
		log.Fatalln(err)
	}

	if err = client.Ping(ctx); err != nil {
		log.Fatalln(err)
	}

	// Disable HTTP metrics to keep the Prometheus output focused on Redis and xredis.
	httpMeterProvider := metricnoop.NewMeterProvider()

	traceHandler := func(name string, handler http.HandlerFunc) http.Handler {
		return otelhttp.NewHandler(
			handler,
			name,
			otelhttp.WithTracerProvider(tracerProvider),
			otelhttp.WithMeterProvider(httpMeterProvider),
			otelhttp.WithPropagators(propagator),
		)
	}

	// Register HTTP handlers.
	mux := http.NewServeMux()
	mux.Handle("GET /healthz", traceHandler("GET /healthz", healthHandler))
	mux.Handle("PUT /values/{key}", traceHandler("PUT /values/{key}", setValueHandler))
	mux.Handle("GET /values/{key}", traceHandler("GET /values/{key}", getValueHandler))
	mux.Handle("DELETE /values/{key}", traceHandler("DELETE /values/{key}", deleteValueHandler))
	mux.Handle("POST /errors/{key}", traceHandler("POST /errors/{key}", redisErrorHandler))
	mux.Handle("GET /metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	server := &http.Server{
		Addr:              httpAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("OpenTelemetry example listening on http://%s", httpAddr)
		log.Printf("Prometheus metrics available at http://%s/metrics", httpAddr)
		log.Printf("Redis address: %s", redisAddr)
		log.Printf("Jaeger UI: http://localhost:16686")

		errCh <- server.ListenAndServe()
	}()

	// Wait for the HTTP server or shutdown signal.
	select {
	case err = <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Println("HTTP server error:", err)
		}

	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		if shutdownErr := server.Shutdown(shutdownCtx); shutdownErr != nil {
			log.Println("unable to shutdown HTTP server:", shutdownErr)
		}
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	if err := client.Ping(r.Context()); err != nil {
		writeError(r.Context(), w, http.StatusServiceUnavailable, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"trace_id": traceID(r.Context()),
	})
}

func setValueHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "store Redis value")
	defer span.End()

	key := r.PathValue("key")
	span.SetAttributes(attribute.String("xredis.example.key", valueKey(key)))

	var req valueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		recordSpanError(span, err)
		writeError(ctx, w, http.StatusBadRequest, err)
		return
	}

	span.SetAttributes(attribute.Int("xredis.example.value_length", len(req.Value)))

	if err := valueCache.Set(ctx, key, req.Value); err != nil {
		recordSpanError(span, err)
		writeError(ctx, w, http.StatusInternalServerError, err)
		return
	}

	span.SetStatus(codes.Ok, "value stored")

	writeJSON(w, http.StatusOK, map[string]any{
		"key":      valueKey(key),
		"trace_id": traceID(ctx),
		"value":    req.Value,
	})
}

func getValueHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "load Redis value")
	defer span.End()

	key := r.PathValue("key")
	span.SetAttributes(attribute.String("xredis.example.key", valueKey(key)))

	value, ok, err := valueCache.Get(ctx, key)
	if err != nil {
		recordSpanError(span, err)
		writeError(ctx, w, http.StatusInternalServerError, err)
		return
	}

	span.SetAttributes(attribute.Bool("xredis.example.found", ok))
	if !ok {
		writeError(ctx, w, http.StatusNotFound, xredis.ErrKeyNotFound)
		return
	}

	span.SetStatus(codes.Ok, "value loaded")

	writeJSON(w, http.StatusOK, map[string]any{
		"key":      valueKey(key),
		"trace_id": traceID(ctx),
		"value":    value,
	})
}

func deleteValueHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "delete Redis value")
	defer span.End()

	key := r.PathValue("key")
	span.SetAttributes(attribute.String("xredis.example.key", valueKey(key)))

	if err := valueCache.Delete(ctx, key); err != nil {
		recordSpanError(span, err)
		writeError(ctx, w, http.StatusInternalServerError, err)
		return
	}

	span.SetStatus(codes.Ok, "value deleted")

	writeJSON(w, http.StatusOK, map[string]any{
		"deleted":  true,
		"key":      valueKey(key),
		"trace_id": traceID(ctx),
	})
}

func redisErrorHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "run failing Redis command")
	defer span.End()

	key := valueKey(r.PathValue("key"))
	span.SetAttributes(attribute.String("xredis.example.key", key))

	if err := client.Set(ctx, key, "not-an-integer", defaultTTL); err != nil {
		recordSpanError(span, err)
		writeError(ctx, w, http.StatusInternalServerError, err)
		return
	}

	_, err := client.Incr(ctx, key)
	if err == nil {
		err = fmt.Errorf("expected Redis INCR to fail")
	}

	recordSpanError(span, err)
	writeError(ctx, w, http.StatusInternalServerError, err)
}

func shutdownClient(client *xredis.Client) {
	if err := client.Close(); err != nil {
		log.Printf("close Redis client: %v", err)
	}
}

func recordSpanError(span trace.Span, err error) {
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

func traceID(ctx context.Context) string {
	spanContext := trace.SpanFromContext(ctx).SpanContext()
	if !spanContext.IsValid() {
		return ""
	}

	return spanContext.TraceID().String()
}

func valueKey(key string) string {
	return keyPrefix + key
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if value != nil {
		_ = json.NewEncoder(w).Encode(value)
	}
}

func writeError(ctx context.Context, w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{
		"error":    err.Error(),
		"trace_id": traceID(ctx),
	})
}

func env(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}
