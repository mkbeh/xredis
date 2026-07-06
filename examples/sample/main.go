package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mkbeh/xredis"
)

const (
	keyPrefix       = "xredis:sample:"
	defaultTTL      = time.Minute
	defaultDB       = 0
	defaultHTTP     = "localhost:8080"
	defaultRedis    = "localhost:6379"
	sampleClient    = "sample-client"
	contentTypeKey  = "Content-Type"
	contentTypeJSON = "application/json"
)

var (
	client *xredis.Client

	redisAddr string
	httpAddr  string
)

func init() {
	redisAddr = env("REDIS_ADDR", defaultRedis)
	httpAddr = env("HTTP_ADDR", defaultHTTP)
}

type MessageRequest struct {
	Value      string `json:"value"`
	TTLSeconds int    `json:"ttl_seconds"`
}

type UserRequest struct {
	Name       string `json:"name"`
	Age        int    `json:"age"`
	Active     bool   `json:"active"`
	TTLSeconds int    `json:"ttl_seconds"`
}

type User struct {
	ID     string `json:"id" redis:"id"`
	Name   string `json:"name" redis:"name"`
	Age    int    `json:"age" redis:"age"`
	Active bool   `json:"active" redis:"active"`
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	if err := client.Ping(r.Context()); err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

func messageHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPut:
		setMessageHandler(w, r)
	case http.MethodGet:
		getMessageHandler(w, r)
	default:
		methodNotAllowed(w)
	}
}

func setMessageHandler(w http.ResponseWriter, r *http.Request) {
	var req MessageRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ttl := ttlFromSeconds(req.TTLSeconds)

	if err := client.Set(r.Context(), messageKey(), req.Value, ttl); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"key":   messageKey(),
		"value": req.Value,
		"ttl":   ttl.String(),
	})
}

func getMessageHandler(w http.ResponseWriter, r *http.Request) {
	value, ok, err := client.String(r.Context(), messageKey())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if !ok {
		http.Error(w, "message not found", http.StatusNotFound)
		return
	}

	ttl, err := client.Raw().TTL(r.Context(), messageKey()).Result()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"key":   messageKey(),
		"value": value,
		"ttl":   ttl.String(),
	})
}

func incrementCounterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	value, err := client.Raw().Incr(r.Context(), counterKey()).Result()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"key":   counterKey(),
		"value": value,
	})
}

func usersHandler(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/users/")
	if id == "" || id == r.URL.Path {
		http.Error(w, "missing user id", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodPut:
		setUserHandler(w, r, id)
	case http.MethodGet:
		getUserHandler(w, r, id)
	default:
		methodNotAllowed(w)
	}
}

func setUserHandler(w http.ResponseWriter, r *http.Request, id string) {
	var req UserRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	user := User{
		ID:     id,
		Name:   req.Name,
		Age:    req.Age,
		Active: req.Active,
	}

	ttl := ttlFromSeconds(req.TTLSeconds)

	if err := client.HSetObject(r.Context(), userKey(id), user, ttl); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"key":  userKey(id),
		"user": user,
		"ttl":  ttl.String(),
	})
}

func getUserHandler(w http.ResponseWriter, r *http.Request, id string) {
	var user User

	if err := client.HGetAll(r.Context(), userKey(id), &user); err != nil {
		if errors.Is(err, xredis.ErrKeyNotFound) {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}

		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ttl, err := client.Raw().TTL(r.Context(), userKey(id)).Result()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"key":  userKey(id),
		"user": user,
		"ttl":  ttl.String(),
	})
}

func cleanupHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		methodNotAllowed(w)
		return
	}

	deleted, err := deleteSampleKeys(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"deleted": deleted,
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
	defer client.Close()

	http.HandleFunc("/healthz", healthHandler)
	http.HandleFunc("/message", messageHandler)
	http.HandleFunc("/counter/increment", incrementCounterHandler)
	http.HandleFunc("/users/", usersHandler)
	http.HandleFunc("/sample", cleanupHandler)

	if err = http.ListenAndServe(httpAddr, nil); err != nil {
		log.Fatalln("unable to start web server:", err)
	}
}

func deleteSampleKeys(ctx context.Context) (int64, error) {
	var keys []string

	iter := client.Raw().Scan(ctx, 0, keyPrefix+"*", 0).Iterator()
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}

	if err := iter.Err(); err != nil {
		return 0, err
	}

	if len(keys) == 0 {
		return 0, nil
	}

	return client.Raw().Del(ctx, keys...).Result()
}

func messageKey() string {
	return keyPrefix + "message"
}

func counterKey() string {
	return keyPrefix + "counter"
}

func userKey(id string) string {
	return keyPrefix + "user:" + id
}

func ttlFromSeconds(seconds int) time.Duration {
	if seconds <= 0 {
		return defaultTTL
	}

	return time.Duration(seconds) * time.Second
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set(contentTypeKey, contentTypeJSON)
	w.WriteHeader(status)

	if value != nil {
		_ = json.NewEncoder(w).Encode(value)
	}
}

func methodNotAllowed(w http.ResponseWriter) {
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func env(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}
