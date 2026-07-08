package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/mkbeh/xredis"
)

const (
	defaultDB    = 0
	defaultHTTP  = "localhost:8080"
	defaultRedis = "localhost:6379"

	valueTTL        = time.Hour
	sampleClient    = "cas-example-client"
	contentTypeKey  = "Content-Type"
	contentTypeJSON = "application/json"
)

var (
	client *xredis.Client
	repo   *orderRepository

	redisAddr string
	httpAddr  string

	sampleIDs = []string{"42", "7", "100"}
)

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

func getStatusHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	key := statusKey(id)

	status, ok, err := client.String(r.Context(), key)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if !ok {
		writeError(w, http.StatusNotFound, xredis.ErrKeyNotFound)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"key":    key,
		"status": status,
	})
}

func seedStatusHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	key := statusKey(id)

	if err := client.Set(r.Context(), key, "processing", valueTTL); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"key":    key,
		"status": "processing",
	})
}

func completeStatusHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	key := statusKey(id)

	swapped, err := client.CompareAndSwap(r.Context(), key, "processing", "completed", valueTTL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if !swapped {
		writeError(w, http.StatusConflict, ErrCompareConditionFailed)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"key":     key,
		"swapped": true,
		"status":  "completed",
	})
}

func deleteProcessingStatusHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	key := statusKey(id)

	deleted, err := client.CompareAndDelete(r.Context(), key, "processing")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if !deleted {
		writeError(w, http.StatusConflict, ErrCompareConditionFailed)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"key":     key,
		"deleted": true,
	})
}

func staleStatusHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	key := statusKey(id)

	if err := client.Set(r.Context(), key, "processing", valueTTL); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	firstSwapped, err := client.CompareAndSwap(r.Context(), key, "processing", "cancelled", valueTTL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	staleSwapped, err := client.CompareAndSwap(r.Context(), key, "processing", "completed", valueTTL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	current, _, err := client.String(r.Context(), key)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"key":            key,
		"first_swapped":  firstSwapped,
		"stale_swapped":  staleSwapped,
		"current_status": current,
	})
}

func getOrderHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	order, ok, err := repo.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if !ok {
		writeError(w, http.StatusNotFound, xredis.ErrKeyNotFound)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"key":   orderKey(id),
		"order": order,
	})
}

func seedOrderHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	order, err := repo.Seed(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"key":   orderKey(id),
		"order": order,
	})
}

func completeOrderHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	order, swapped, err := repo.Complete(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if !swapped {
		writeError(w, http.StatusConflict, ErrCompareConditionFailed)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"key":     orderKey(id),
		"swapped": true,
		"order":   order,
	})
}

func deleteCurrentOrderHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	expected, deleted, err := repo.DeleteIfCurrent(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if !deleted {
		writeError(w, http.StatusConflict, ErrCompareConditionFailed)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"key":            orderKey(id),
		"deleted":        true,
		"expected_order": expected,
	})
}

func staleOrderHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	result, err := repo.StaleSwap(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func cleanupHandler(w http.ResponseWriter, r *http.Request) {
	keys := make([]string, 0, len(sampleIDs)*2)
	for _, id := range sampleIDs {
		keys = append(keys, statusKey(id), orderKey(id))
	}

	if err := client.MassDelete(r.Context(), keys); err != nil {
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

	repo = newOrderRepository(client, valueTTL)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthHandler)
	mux.HandleFunc("GET /statuses/{id}", getStatusHandler)
	mux.HandleFunc("POST /statuses/{id}/seed", seedStatusHandler)
	mux.HandleFunc("POST /statuses/{id}/complete", completeStatusHandler)
	mux.HandleFunc("DELETE /statuses/{id}/processing", deleteProcessingStatusHandler)
	mux.HandleFunc("POST /statuses/{id}/stale", staleStatusHandler)
	mux.HandleFunc("GET /orders/{id}", getOrderHandler)
	mux.HandleFunc("POST /orders/{id}/seed", seedOrderHandler)
	mux.HandleFunc("POST /orders/{id}/complete", completeOrderHandler)
	mux.HandleFunc("DELETE /orders/{id}/current", deleteCurrentOrderHandler)
	mux.HandleFunc("POST /orders/{id}/stale", staleOrderHandler)
	mux.HandleFunc("DELETE /sample", cleanupHandler)

	server := &http.Server{
		Addr:              httpAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("cas example listening on http://%s", httpAddr)
	log.Printf("redis address: %s", redisAddr)

	if err = server.ListenAndServe(); err != nil {
		log.Fatalln("unable to start web server:", err)
	}
}

func statusKey(id string) string {
	return "xredis:cas:status:" + id
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set(contentTypeKey, contentTypeJSON)
	w.WriteHeader(status)

	if value != nil {
		_ = json.NewEncoder(w).Encode(value)
	}
}

func writeError(w http.ResponseWriter, status int, err error) {
	if errors.Is(err, xredis.ErrKeyNotFound) {
		status = http.StatusNotFound
	}

	writeJSON(w, status, map[string]string{
		"error": err.Error(),
	})
}

func env(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}
