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

	simpleLockTTL     = 5 * time.Second
	shortLockTTL      = 1 * time.Second
	extendedLockTTL   = 5 * time.Second
	fencedLockTTL     = 5 * time.Second
	fencingCounterTTL = 7 * 24 * time.Hour
	unlockTimeout     = time.Second
	sampleClient      = "locks-example-client"
	contentTypeKey    = "Content-Type"
	contentTypeJSON   = "application/json"
)

var (
	client *xredis.Client
	repo   *orderRepository

	redisAddr string
	httpAddr  string

	sampleOrderIDs = []string{"42", "7", "100"}
)

var errLockNotAcquired = errors.New("lock not acquired")

type unlocker interface {
	Unlock(ctx context.Context) error
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

func getOrderHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	order, ok := repo.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, xredis.ErrKeyNotFound)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"order": order,
	})
}

func simpleLockWorkHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	key := simpleLockKey(id)

	lock, acquired, err := client.TryLock(r.Context(), key, simpleLockTTL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if !acquired {
		writeError(w, http.StatusConflict, errLockNotAcquired)
		return
	}
	defer unlockSafely(lock)

	order, err := repo.Process(r.Context(), id, "processed_with_lock")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"lock_key": key,
		"token":    lock.Token(),
		"order":    order,
	})
}

func extendLockHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	key := simpleLockKey(id)

	lock, acquired, err := client.TryLock(r.Context(), key, shortLockTTL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if !acquired {
		writeError(w, http.StatusConflict, errLockNotAcquired)
		return
	}
	defer unlockSafely(lock)

	if err = sleepContext(r.Context(), shortLockTTL/2); err != nil {
		writeError(w, http.StatusRequestTimeout, err)
		return
	}

	extended, err := lock.Extend(r.Context(), extendedLockTTL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if !extended {
		writeError(w, http.StatusConflict, xredis.ErrLockNotOwned)
		return
	}

	order, err := repo.Process(r.Context(), id, "processed_after_extend")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"lock_key": key,
		"token":    lock.Token(),
		"extended": true,
		"order":    order,
	})
}

func fencedLockWorkHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	key := fencedLockKey(id)
	fenceKey := fencingKey(id)

	lock, acquired, err := client.TryFencedLock(
		r.Context(),
		key,
		fenceKey,
		fencedLockTTL,
		xredis.WithFencingCounterTTL(fencingCounterTTL),
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if !acquired {
		writeError(w, http.StatusConflict, errLockNotAcquired)
		return
	}
	defer unlockSafely(lock)

	order, err := repo.ProcessWithFence(
		r.Context(),
		id,
		lock.FencingToken(),
		"processed_with_fencing",
	)
	if err != nil {
		if errors.Is(err, ErrStaleFencingToken) {
			writeError(w, http.StatusConflict, err)
			return
		}

		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"lock_key":      key,
		"fencing_key":   fenceKey,
		"token":         lock.Token(),
		"fencing_token": lock.FencingToken(),
		"order":         order,
	})
}

func staleFencingTokenHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	key := fencedLockKey(id)
	fenceKey := fencingKey(id)

	firstLock, acquired, err := client.TryFencedLock(
		r.Context(),
		key,
		fenceKey,
		fencedLockTTL,
		xredis.WithFencingCounterTTL(fencingCounterTTL),
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if !acquired {
		writeError(w, http.StatusConflict, errLockNotAcquired)
		return
	}

	staleToken := firstLock.FencingToken()

	if err = firstLock.Unlock(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	secondLock, acquired, err := client.TryFencedLock(
		r.Context(),
		key,
		fenceKey,
		fencedLockTTL,
		xredis.WithFencingCounterTTL(fencingCounterTTL),
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if !acquired {
		writeError(w, http.StatusConflict, errLockNotAcquired)
		return
	}
	defer unlockSafely(secondLock)

	order, err := repo.ProcessWithFence(
		r.Context(),
		id,
		secondLock.FencingToken(),
		"processed_with_fresh_fencing_token",
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	_, err = repo.ProcessWithFence(
		r.Context(),
		id,
		staleToken,
		"processed_with_stale_fencing_token",
	)
	if !errors.Is(err, ErrStaleFencingToken) {
		writeError(w, http.StatusInternalServerError, errors.New("expected stale fencing token rejection"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"lock_key":             key,
		"fencing_key":          fenceKey,
		"stale_fencing_token":  staleToken,
		"fresh_fencing_token":  secondLock.FencingToken(),
		"stale_token_rejected": true,
		"order":                order,
	})
}

func cleanupHandler(w http.ResponseWriter, r *http.Request) {
	var keys []string

	for _, id := range sampleOrderIDs {
		keys = append(keys,
			simpleLockKey(id),
			fencedLockKey(id),
			fencingKey(id),
		)
	}

	if err := client.DeleteMany(r.Context(), keys); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	repo.Reset()

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

	repo = newOrderRepository()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthHandler)
	mux.HandleFunc("GET /orders/{id}", getOrderHandler)
	mux.HandleFunc("POST /locks/{id}/work", simpleLockWorkHandler)
	mux.HandleFunc("POST /locks/{id}/extend", extendLockHandler)
	mux.HandleFunc("POST /orders/{id}/process", fencedLockWorkHandler)
	mux.HandleFunc("POST /orders/{id}/stale", staleFencingTokenHandler)
	mux.HandleFunc("DELETE /sample", cleanupHandler)

	server := &http.Server{
		Addr:              httpAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("locks example listening on http://%s", httpAddr)
	log.Printf("redis address: %s", redisAddr)

	if err = server.ListenAndServe(); err != nil {
		log.Fatalln("unable to start web server:", err)
	}
}

func unlockSafely(lock unlocker) {
	ctx, cancel := context.WithTimeout(context.Background(), unlockTimeout)
	defer cancel()

	if err := lock.Unlock(ctx); err != nil && !errors.Is(err, xredis.ErrLockNotOwned) {
		log.Println("unable to unlock:", err)
	}
}

func simpleLockKey(id string) string {
	return "xredis:locks:simple:" + id
}

func fencedLockKey(id string) string {
	return "xredis:locks:{order:" + id + "}"
}

func fencingKey(id string) string {
	return "xredis:fencing:{order:" + id + "}"
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
