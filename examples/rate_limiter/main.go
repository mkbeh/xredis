package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/mkbeh/xredis"
)

const (
	defaultDB    = 0
	defaultHTTP  = "localhost:8080"
	defaultRedis = "localhost:6379"

	rateLimitPrefix = "xredis:rate_limiter:"

	rateLimitWindow = 30 * time.Second

	fixedWindowLimit   int64 = 5
	slidingWindowLimit int64 = 5
	tokenBucketLimit   int64 = 5
	tokenBucketBurst   int64 = 10

	sampleClient    = "rate-limiter-example-client"
	contentTypeKey  = "Content-Type"
	contentTypeJSON = "application/json"
)

var (
	client  *xredis.Client
	limiter *xredis.RateLimiter

	redisAddr string
	httpAddr  string

	sampleIDs = []string{"42", "7", "100"}
)

type rateLimitResponse struct {
	Algorithm         string `json:"algorithm"`
	Key               string `json:"key"`
	Allowed           bool   `json:"allowed"`
	Limit             int64  `json:"limit"`
	Remaining         int64  `json:"remaining"`
	RetryAfter        string `json:"retry_after"`
	RetryAfterSeconds int64  `json:"retry_after_seconds"`
	ResetAfter        string `json:"reset_after"`
	ResetAfterSeconds int64  `json:"reset_after_seconds"`
}

func init() {
	redisAddr = env("REDIS_ADDR", defaultRedis)
	httpAddr = env("HTTP_ADDR", defaultHTTP)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	if err := client.Ping(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

func fixedWindowHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	key := fixedWindowKey(id)

	decision, err := limiter.AllowFixedWindow(r.Context(), key, xredis.RateLimit{
		Limit:  fixedWindowLimit,
		Window: rateLimitWindow,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeDecision(w, "fixed_window", key, decision)
}

func defaultAllowHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	key := fixedWindowKey(id)

	decision, err := limiter.Allow(r.Context(), key, xredis.RateLimit{
		Limit:  fixedWindowLimit,
		Window: rateLimitWindow,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeDecision(w, "fixed_window", key, decision)
}

func slidingWindowHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	key := slidingWindowKey(id)

	decision, err := limiter.AllowSlidingWindow(r.Context(), key, xredis.RateLimit{
		Limit:  slidingWindowLimit,
		Window: rateLimitWindow,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeDecision(w, "sliding_window", key, decision)
}

func tokenBucketHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	key := tokenBucketKey(id)

	decision, err := limiter.AllowTokenBucket(r.Context(), key, xredis.TokenBucketRateLimit{
		Limit:  tokenBucketLimit,
		Window: rateLimitWindow,
		Burst:  tokenBucketBurst,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeDecision(w, "token_bucket", key, decision)
}

func cleanupHandler(w http.ResponseWriter, r *http.Request) {
	keys := make([]string, 0, len(sampleIDs)*3)

	for _, id := range sampleIDs {
		keys = append(keys,
			rateLimitPrefix+fixedWindowKey(id),
			rateLimitPrefix+slidingWindowKey(id),
			rateLimitPrefix+tokenBucketKey(id),
		)
	}

	if err := client.DeleteMany(r.Context(), keys); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "reset",
	})
}

func main() {
	var err error

	client, err = xredis.NewClient(
		xredis.WithClientConfig(&xredis.ClientConfig{
			Addr: redisAddr,
			DB:   defaultDB,
		}),
		xredis.WithClientID(sampleClient),
	)
	if err != nil {
		log.Fatalln(err)
	}
	defer func() {
		if closeErr := client.Close(); closeErr != nil {
			log.Println("unable to close redis client:", closeErr)
		}
	}()

	if err = client.Ping(context.Background()); err != nil {
		log.Fatalln(err)
	}

	limiter, err = client.RateLimiter(
		xredis.WithRateLimiterPrefix(rateLimitPrefix),
	)
	if err != nil {
		log.Fatalln(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthHandler)
	mux.HandleFunc("POST /allow/{id}", defaultAllowHandler)
	mux.HandleFunc("POST /fixed-window/{id}", fixedWindowHandler)
	mux.HandleFunc("POST /sliding-window/{id}", slidingWindowHandler)
	mux.HandleFunc("POST /token-bucket/{id}", tokenBucketHandler)
	mux.HandleFunc("DELETE /sample", cleanupHandler)

	server := &http.Server{
		Addr:              httpAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("rate limiter example listening on http://%s", httpAddr)
	log.Printf("redis address: %s", redisAddr)

	if err = server.ListenAndServe(); err != nil {
		log.Fatalln("unable to start web server:", err)
	}
}

func fixedWindowKey(id string) string {
	return "fixed:user:" + id
}

func slidingWindowKey(id string) string {
	return "sliding:user:" + id
}

func tokenBucketKey(id string) string {
	return "bucket:user:" + id
}

func writeDecision(w http.ResponseWriter, algorithm, key string, decision xredis.RateLimitDecision) {
	writeRateLimitHeaders(w, decision)

	status := http.StatusOK
	if !decision.Allowed {
		status = http.StatusTooManyRequests
	}

	writeJSON(w, status, rateLimitResponse{
		Algorithm:         algorithm,
		Key:               key,
		Allowed:           decision.Allowed,
		Limit:             decision.Limit,
		Remaining:         decision.Remaining,
		RetryAfter:        durationString(decision.RetryAfter),
		RetryAfterSeconds: durationSeconds(decision.RetryAfter),
		ResetAfter:        durationString(decision.ResetAfter),
		ResetAfterSeconds: durationSeconds(decision.ResetAfter),
	})
}

func writeRateLimitHeaders(w http.ResponseWriter, decision xredis.RateLimitDecision) {
	w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(decision.Limit, 10))
	w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(decision.Remaining, 10))
	w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(durationSeconds(decision.ResetAfter), 10))

	if !decision.Allowed {
		w.Header().Set("Retry-After", strconv.FormatInt(durationSeconds(decision.RetryAfter), 10))
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set(contentTypeKey, contentTypeJSON)
	w.WriteHeader(status)

	if value != nil {
		_ = json.NewEncoder(w).Encode(value)
	}
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{
		"error": err.Error(),
	})
}

func durationString(d time.Duration) string {
	if d <= 0 {
		return "0s"
	}

	return d.Round(time.Millisecond).String()
}

func durationSeconds(d time.Duration) int64 {
	if d <= 0 {
		return 0
	}

	return int64((d + time.Second - 1) / time.Second)
}

func env(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}
