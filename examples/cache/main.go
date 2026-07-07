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
	keyPrefix        = "xredis:cache:"
	defaultDB        = 0
	defaultHTTP      = "localhost:8080"
	defaultRedis     = "localhost:6379"
	cacheTTL         = 60 * time.Second
	cacheJitter      = 5 * time.Second
	cacheNegativeTTL = 30 * time.Second
	sampleClient     = "cache-example-client"
	contentTypeKey   = "Content-Type"
	contentTypeJSON  = "application/json"
)

var (
	client    *xredis.Client
	userCache *xredis.Cache[User]
	repo      *userRepository

	redisAddr string
	httpAddr  string

	sampleUserIDs = []string{"42", "7", "100", "404"}
)

func init() {
	redisAddr = env("REDIS_ADDR", defaultRedis)
	httpAddr = env("HTTP_ADDR", defaultHTTP)
}

type UserRequest struct {
	Name   string `json:"name"`
	Age    int    `json:"age"`
	Active bool   `json:"active"`
}

type User struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Age    int    `json:"age"`
	Active bool   `json:"active"`
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

func setUserHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req UserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	user := User{
		ID:     id,
		Name:   req.Name,
		Age:    req.Age,
		Active: req.Active,
	}

	repo.Set(user)

	if err := userCache.Set(r.Context(), id, user); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"key":    userKey(id),
		"source": "write_through",
		"user":   user,
	})
}

func getCachedUserHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	user, ok, err := userCache.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if !ok {
		writeError(w, http.StatusNotFound, xredis.ErrKeyNotFound)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"key":    userKey(id),
		"source": "cache",
		"user":   user,
	})
}

func getOrLoadUserHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	user, err := userCache.GetOrLoad(r.Context(), id, func(ctx context.Context) (User, error) {
		return repo.GetByID(ctx, id)
	})
	if err != nil {
		if errors.Is(err, xredis.ErrKeyNotFound) {
			writeError(w, http.StatusNotFound, err)
			return
		}

		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"key":    userKey(id),
		"source": "cache_or_loader",
		"user":   user,
	})
}

func deleteUserHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	repo.Delete(id)

	if err := userCache.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"key":    userKey(id),
		"status": "deleted",
	})
}

func statsHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"repository_loads": repo.Loads(),
	})
}

func cleanupHandler(w http.ResponseWriter, r *http.Request) {
	if err := deleteSampleUsers(r.Context()); err != nil {
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

	userCache, err = xredis.NewCache[User](
		client,
		xredis.WithCachePrefix(keyPrefix+"user:"),
		xredis.WithCacheTTL(cacheTTL),
		xredis.WithCacheJitter(cacheJitter),
		xredis.WithCacheNegativeTTL(cacheNegativeTTL),
	)
	if err != nil {
		log.Fatalln(err)
	}

	repo = newUserRepository()

	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", healthHandler)

	mux.HandleFunc("PUT /users/{id}", setUserHandler)
	mux.HandleFunc("GET /users/{id}", getCachedUserHandler)
	mux.HandleFunc("GET /users/{id}/load", getOrLoadUserHandler)
	mux.HandleFunc("DELETE /users/{id}", deleteUserHandler)

	mux.HandleFunc("GET /stats", statsHandler)
	mux.HandleFunc("DELETE /sample", cleanupHandler)

	server := &http.Server{
		Addr:              httpAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	if err = server.ListenAndServe(); err != nil {
		log.Fatalln("unable to start web server:", err)
	}
}

func deleteSampleUsers(ctx context.Context) error {
	for _, id := range sampleUserIDs {
		if err := userCache.Delete(ctx, id); err != nil {
			return err
		}
	}

	return nil
}

func userKey(id string) string {
	return keyPrefix + "user:" + id
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
