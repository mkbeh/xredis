package main

import (
	"context"
	"encoding/json"
	"fmt"
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
	defaultTTL   = 10 * time.Minute

	keyPrefix    = "xredis:commands:"
	sampleClient = "commands-example-client"

	contentTypeKey  = "Content-Type"
	contentTypeJSON = "application/json"
)

var (
	client *xredis.Client

	redisAddr string
	httpAddr  string
)

type messageRequest struct {
	Value      string `json:"value"`
	TTLSeconds int    `json:"ttl_seconds"`
}

type profileRequest struct {
	Name       string `json:"name"`
	Email      string `json:"email"`
	Active     bool   `json:"active"`
	TTLSeconds int    `json:"ttl_seconds"`
}

type profile struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Active bool   `json:"active"`
}

type userRequest struct {
	Name       string `json:"name"`
	Active     bool   `json:"active"`
	TTLSeconds int    `json:"ttl_seconds"`
}

type userHash struct {
	ID     string `json:"id" redis:"id"`
	Name   string `json:"name" redis:"name"`
	Active bool   `json:"active" redis:"active"`
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

func setMessageHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req messageRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	ttl, err := ttlFromSeconds(req.TTLSeconds)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if err = client.Set(r.Context(), messageKey(id), req.Value, ttl); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"key":   messageKey(id),
		"value": req.Value,
		"ttl":   ttl.String(),
	})
}

func getMessageHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	value, ok, err := client.String(r.Context(), messageKey(id))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Errorf("message not found"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"key":   messageKey(id),
		"value": value,
	})
}

func setProfileHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req profileRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	ttl, err := ttlFromSeconds(req.TTLSeconds)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	value := profile{
		ID:     id,
		Name:   req.Name,
		Email:  req.Email,
		Active: req.Active,
	}

	if err = client.SetStruct(r.Context(), profileKey(id), value, ttl); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"key":     profileKey(id),
		"profile": value,
		"ttl":     ttl.String(),
	})
}

func getProfileHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var value profile
	ok, err := client.GetStruct(r.Context(), profileKey(id), &value)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Errorf("profile not found"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"key":     profileKey(id),
		"profile": value,
	})
}

func setUserHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req userRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	ttl, err := ttlFromSeconds(req.TTLSeconds)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	value := userHash{
		ID:     id,
		Name:   req.Name,
		Active: req.Active,
	}

	if err = client.HSet(r.Context(), userKey(id), ttl, value); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"key":  userKey(id),
		"user": value,
		"ttl":  ttl.String(),
	})
}

func getUserHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var value userHash
	ok, err := client.HGetAll(r.Context(), userKey(id), &value)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Errorf("user hash not found"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"key":  userKey(id),
		"user": value,
	})
}

func incrementCounterHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := client.Incr(r.Context(), counterKey(id)); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	value, ok, err := client.Int64(r.Context(), counterKey(id))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Errorf("counter not found"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"key":   counterKey(id),
		"value": value,
	})
}

func cleanupHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	keys := []string{
		messageKey(id),
		profileKey(id),
		userKey(id),
		counterKey(id),
	}

	for _, key := range keys {
		if err := client.Delete(r.Context(), key); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "reset",
		"id":     id,
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
			log.Println("unable to close Redis client:", closeErr)
		}
	}()

	if err = client.Ping(context.Background()); err != nil {
		log.Fatalln(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthHandler)

	mux.HandleFunc("PUT /messages/{id}", setMessageHandler)
	mux.HandleFunc("GET /messages/{id}", getMessageHandler)

	mux.HandleFunc("PUT /profiles/{id}", setProfileHandler)
	mux.HandleFunc("GET /profiles/{id}", getProfileHandler)

	mux.HandleFunc("PUT /users/{id}", setUserHandler)
	mux.HandleFunc("GET /users/{id}", getUserHandler)

	mux.HandleFunc("POST /counters/{id}/increment", incrementCounterHandler)

	mux.HandleFunc("DELETE /sample/{id}", cleanupHandler)

	server := &http.Server{
		Addr:              httpAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("commands example listening on http://%s", httpAddr)
	log.Printf("redis address: %s", redisAddr)

	if err = server.ListenAndServe(); err != nil {
		log.Fatalln("unable to start web server:", err)
	}
}

func messageKey(id string) string {
	return keyPrefix + "message:" + id
}

func profileKey(id string) string {
	return keyPrefix + "profile:" + id
}

func userKey(id string) string {
	return keyPrefix + "user:" + id
}

func counterKey(id string) string {
	return keyPrefix + "counter:" + id
}

func ttlFromSeconds(seconds int) (time.Duration, error) {
	if seconds < 0 {
		return 0, fmt.Errorf("ttl_seconds must not be negative")
	}

	if seconds == 0 {
		return defaultTTL, nil
	}

	return time.Duration(seconds) * time.Second, nil
}

func decodeJSON(r *http.Request, dst any) error {
	if r.Body == nil {
		return fmt.Errorf("request body is required")
	}

	return json.NewDecoder(r.Body).Decode(dst)
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
