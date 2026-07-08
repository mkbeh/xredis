package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mkbeh/xredis"
	rdb "github.com/redis/go-redis/v9"
)

const (
	defaultDB    = 0
	defaultHTTP  = "localhost:8080"
	defaultRedis = "localhost:6379"

	samplePrefix = "xredis:scan:"
	sampleTTL    = 10 * time.Minute

	defaultScanCount int64 = 100

	sampleClient    = "scan-example-client"
	contentTypeKey  = "Content-Type"
	contentTypeJSON = "application/json"
)

var (
	client *xredis.Client

	redisAddr string
	httpAddr  string

	sampleIDs = []string{"42", "7", "100"}
)

type scanPageResponse struct {
	Match      string   `json:"match"`
	Type       string   `json:"type,omitempty"`
	Count      int64    `json:"count"`
	Cursor     uint64   `json:"cursor"`
	NextCursor uint64   `json:"next_cursor"`
	Keys       []string `json:"keys"`
}

type scanAllResponse struct {
	Match string   `json:"match"`
	Type  string   `json:"type,omitempty"`
	Count int64    `json:"count"`
	Total int      `json:"total"`
	Keys  []string `json:"keys"`
}

type scanBatchResponse struct {
	Match   string      `json:"match"`
	Type    string      `json:"type,omitempty"`
	Count   int64       `json:"count"`
	Total   int         `json:"total"`
	Batches []scanBatch `json:"batches"`
}

type scanBatch struct {
	Index int      `json:"index"`
	Total int      `json:"total"`
	Keys  []string `json:"keys"`
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

func seedHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	keys, err := seedSample(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "seeded",
		"keys":   keys,
	})
}

func scanPageHandler(w http.ResponseWriter, r *http.Request) {
	opts, err := scanOptionsFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	keys, nextCursor, err := client.Scan(r.Context(), opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, scanPageResponse{
		Match:      opts.Match,
		Type:       opts.Type,
		Count:      opts.Count,
		Cursor:     opts.Cursor,
		NextCursor: nextCursor,
		Keys:       keys,
	})
}

func scanAllHandler(w http.ResponseWriter, r *http.Request) {
	opts, err := scanOptionsFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	keys, err := client.ScanAll(r.Context(), opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, scanAllResponse{
		Match: opts.Match,
		Type:  opts.Type,
		Count: opts.Count,
		Total: len(keys),
		Keys:  keys,
	})
}

func scanBatchesHandler(w http.ResponseWriter, r *http.Request) {
	opts, err := scanOptionsFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var (
		total   int
		batches []scanBatch
	)

	err = client.ScanEachBatch(r.Context(), opts, func(_ context.Context, keys []string) error {
		batchKeys := append([]string(nil), keys...)

		total += len(batchKeys)
		batches = append(batches, scanBatch{
			Index: len(batches) + 1,
			Total: len(batchKeys),
			Keys:  batchKeys,
		})

		return nil
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, scanBatchResponse{
		Match:   opts.Match,
		Type:    opts.Type,
		Count:   opts.Count,
		Total:   total,
		Batches: batches,
	})
}

func deleteHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	match := deletePattern(id)
	if err := client.ScanDelete(r.Context(), xredis.ScanOptions{
		Match: match,
		Count: defaultScanCount,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "deleted",
		"match":  match,
	})
}

func unlinkHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	match := unlinkPattern(id)
	if err := client.ScanUnlink(r.Context(), xredis.ScanOptions{
		Match: match,
		Count: defaultScanCount,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "unlinked",
		"match":  match,
	})
}

func cleanupHandler(w http.ResponseWriter, r *http.Request) {
	if err := client.ScanDelete(r.Context(), xredis.ScanOptions{
		Match: samplePrefix + "*",
		Count: defaultScanCount,
	}); err != nil {
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

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthHandler)
	mux.HandleFunc("POST /sample/{id}", seedHandler)
	mux.HandleFunc("GET /scan/page", scanPageHandler)
	mux.HandleFunc("GET /scan/all", scanAllHandler)
	mux.HandleFunc("GET /scan/batches", scanBatchesHandler)
	mux.HandleFunc("DELETE /scan/delete/{id}", deleteHandler)
	mux.HandleFunc("DELETE /scan/unlink/{id}", unlinkHandler)
	mux.HandleFunc("DELETE /sample", cleanupHandler)

	server := &http.Server{
		Addr:              httpAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("scan example listening on http://%s", httpAddr)
	log.Printf("redis address: %s", redisAddr)

	if err = server.ListenAndServe(); err != nil {
		log.Fatalln("unable to start web server:", err)
	}
}

func seedSample(ctx context.Context, id string) ([]string, error) {
	keys := make([]string, 0, 15)

	for i := 1; i <= 3; i++ {
		key := stringKey(id, i)
		if err := client.Set(ctx, key, fmt.Sprintf("value-%s-%d", id, i), sampleTTL); err != nil {
			return nil, err
		}

		keys = append(keys, key)
	}

	hashKey := hashKey(id)
	if err := client.HSet(ctx, hashKey, "id", id, sampleTTL); err != nil {
		return nil, err
	}
	if err := client.HSet(ctx, hashKey, "status", "active", sampleTTL); err != nil {
		return nil, err
	}
	keys = append(keys, hashKey)

	streamKey := streamKey(id)
	if _, err := client.Raw().XAdd(ctx, &rdb.XAddArgs{
		Stream: streamKey,
		Values: map[string]any{
			"id":    id,
			"event": "sample_seeded",
		},
	}).Result(); err != nil {
		return nil, err
	}
	keys = append(keys, streamKey)

	for i := 1; i <= 5; i++ {
		key := deleteKey(id, i)
		if err := client.Set(ctx, key, strings.Repeat("delete-value", 10), sampleTTL); err != nil {
			return nil, err
		}

		keys = append(keys, key)
	}

	for i := 1; i <= 5; i++ {
		key := unlinkKey(id, i)
		if err := client.Set(ctx, key, strings.Repeat("unlink-value", 100), sampleTTL); err != nil {
			return nil, err
		}

		keys = append(keys, key)
	}

	return keys, nil
}

func scanOptionsFromRequest(r *http.Request) (xredis.ScanOptions, error) {
	query := r.URL.Query()

	match := query.Get("match")
	if match == "" {
		match = samplePrefix + "*"
	}

	count := defaultScanCount
	if rawCount := query.Get("count"); rawCount != "" {
		parsedCount, err := strconv.ParseInt(rawCount, 10, 64)
		if err != nil {
			return xredis.ScanOptions{}, fmt.Errorf("invalid count: %w", err)
		}

		count = parsedCount
	}

	var cursor uint64
	if rawCursor := query.Get("cursor"); rawCursor != "" {
		parsedCursor, err := strconv.ParseUint(rawCursor, 10, 64)
		if err != nil {
			return xredis.ScanOptions{}, fmt.Errorf("invalid cursor: %w", err)
		}

		cursor = parsedCursor
	}

	return xredis.ScanOptions{
		Cursor: cursor,
		Match:  match,
		Count:  count,
		Type:   query.Get("type"),
	}, nil
}

func stringKey(id string, n int) string {
	return fmt.Sprintf("%sstring:%s:%d", samplePrefix, id, n)
}

func hashKey(id string) string {
	return samplePrefix + "hash:" + id
}

func streamKey(id string) string {
	return samplePrefix + "stream:" + id
}

func deleteKey(id string, n int) string {
	return fmt.Sprintf("%sdelete:%s:%d", samplePrefix, id, n)
}

func unlinkKey(id string, n int) string {
	return fmt.Sprintf("%sunlink:%s:%d", samplePrefix, id, n)
}

func deletePattern(id string) string {
	return fmt.Sprintf("%sdelete:%s:*", samplePrefix, id)
}

func unlinkPattern(id string) string {
	return fmt.Sprintf("%sunlink:%s:*", samplePrefix, id)
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

func env(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}
