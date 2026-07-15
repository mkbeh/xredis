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
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
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

	contentTypeKey  = "Content-Type"
	contentTypeJSON = "application/json"
)

var (
	client *xredis.Client
	tracer trace.Tracer

	redisAddr      string
	httpAddr       string
	serviceName    string
	tracesEndpoint string
)

type valueRequest struct {
	Value string `json:"value"`
}

func init() {
	redisAddr = env("REDIS_ADDR", defaultRedis)
	httpAddr = env("HTTP_ADDR", defaultHTTP)
	serviceName = env("OTEL_SERVICE_NAME", defaultServiceName)
	tracesEndpoint = env(
		"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT",
		defaultTracesEndpoint,
	)
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

	key := valueKey(r.PathValue("key"))
	span.SetAttributes(attribute.String("xredis.example.key", key))

	var req valueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		recordSpanError(span, err)
		writeError(ctx, w, http.StatusBadRequest, err)
		return
	}

	span.SetAttributes(attribute.Int("xredis.example.value_length", len(req.Value)))

	if err := client.Set(ctx, key, req.Value, defaultTTL); err != nil {
		recordSpanError(span, err)
		writeError(ctx, w, http.StatusInternalServerError, err)
		return
	}

	span.SetStatus(codes.Ok, "value stored")

	writeJSON(w, http.StatusOK, map[string]any{
		"key":      key,
		"trace_id": traceID(ctx),
		"value":    req.Value,
	})
}

func getValueHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "load Redis value")
	defer span.End()

	key := valueKey(r.PathValue("key"))
	span.SetAttributes(attribute.String("xredis.example.key", key))

	value, ok, err := client.String(ctx, key)
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
		"key":      key,
		"trace_id": traceID(ctx),
		"value":    value,
	})
}

func deleteValueHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "delete Redis value")
	defer span.End()

	key := valueKey(r.PathValue("key"))
	span.SetAttributes(attribute.String("xredis.example.key", key))

	if err := client.Delete(ctx, key); err != nil {
		recordSpanError(span, err)
		writeError(ctx, w, http.StatusInternalServerError, err)
		return
	}

	span.SetStatus(codes.Ok, "value deleted")

	writeJSON(w, http.StatusOK, map[string]any{
		"deleted":  true,
		"key":      key,
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

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	tracerProvider, err := newTracerProvider(ctx)
	if err != nil {
		log.Fatalln(err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		if shutdownErr := tracerProvider.Shutdown(shutdownCtx); shutdownErr != nil {
			log.Println("unable to shutdown tracer provider:", shutdownErr)
		}
	}()

	propagator := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)

	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(propagator)
	tracer = tracerProvider.Tracer("github.com/mkbeh/xredis/examples/otel")

	client, err = xredis.NewClient(
		xredis.WithClientConfig(&xredis.ClientConfig{
			Addr: redisAddr,
			DB:   defaultDB,
		}),
		xredis.WithClientID(sampleClient),
		xredis.WithTracerProvider(tracerProvider),
		xredis.WithTracingDBStatement(true),
		xredis.WithTracingCallerEnabled(true),
		xredis.WithTracingAttributes(
			attribute.String("xredis.example", "otel"),
		),
	)
	if err != nil {
		log.Fatalln(err)
	}
	defer func() {
		if closeErr := client.Close(); closeErr != nil {
			log.Println("unable to close Redis client:", closeErr)
		}
	}()

	if err = client.Ping(ctx); err != nil {
		log.Fatalln(err)
	}

	traceHandler := func(name string, handler http.HandlerFunc) http.Handler {
		return otelhttp.NewHandler(
			handler,
			name,
			otelhttp.WithTracerProvider(tracerProvider),
			otelhttp.WithPropagators(propagator),
		)
	}

	mux := http.NewServeMux()
	mux.Handle("GET /healthz", traceHandler("GET /healthz", healthHandler))
	mux.Handle("PUT /values/{key}", traceHandler("PUT /values/{key}", setValueHandler))
	mux.Handle("GET /values/{key}", traceHandler("GET /values/{key}", getValueHandler))
	mux.Handle("DELETE /values/{key}", traceHandler("DELETE /values/{key}", deleteValueHandler))
	mux.Handle("POST /errors/{key}", traceHandler("POST /errors/{key}", redisErrorHandler))

	server := &http.Server{
		Addr:              httpAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("OpenTelemetry example listening on http://%s", httpAddr)
		log.Printf("Redis address: %s", redisAddr)
		log.Printf("Jaeger UI: http://localhost:16686")

		errCh <- server.ListenAndServe()
	}()

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

func newTracerProvider(ctx context.Context) (*sdktrace.TracerProvider, error) {
	exporter, err := otlptracehttp.New(
		ctx,
		otlptracehttp.WithEndpointURL(tracesEndpoint),
	)
	if err != nil {
		return nil, err
	}

	res, err := resource.New(
		ctx,
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
		resource.WithAttributes(
			attribute.String("service.name", serviceName),
		),
	)
	if err != nil {
		return nil, err
	}

	return sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	), nil
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
	w.Header().Set(contentTypeKey, contentTypeJSON)
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
