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

	samplePrefix = "xredis:pipeline:"
	sampleTTL    = 10 * time.Minute

	sampleClient    = "pipeline-example-client"
	contentTypeKey  = "Content-Type"
	contentTypeJSON = "application/json"
)

var (
	client *xredis.Client

	redisAddr string
	httpAddr  string

	sampleIDs = []string{"42", "7", "100"}
)

type userProfile struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type userSettings struct {
	Theme         string `json:"theme"`
	Notifications bool   `json:"notifications"`
}

type sampleValue struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	Index int    `json:"index"`
}

type seedResponse struct {
	Status string   `json:"status"`
	Keys   []string `json:"keys"`
}

type removeResponse struct {
	Status string   `json:"status"`
	Keys   []string `json:"keys"`
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

	writeJSON(w, http.StatusOK, seedResponse{
		Status: "seeded",
		Keys:   keys,
	})
}

func deleteHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	keys := deleteKeys(id)

	if err := client.DeleteMany(r.Context(), keys); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, removeResponse{
		Status: "deleted",
		Keys:   keys,
	})
}

func unlinkHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	keys := unlinkKeys(id)

	if err := client.UnlinkMany(r.Context(), keys); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, removeResponse{
		Status: "unlinked",
		Keys:   keys,
	})
}

func cleanupHandler(w http.ResponseWriter, r *http.Request) {
	keys := cleanupKeys()

	if err := client.DeleteMany(r.Context(), keys); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, removeResponse{
		Status: "reset",
		Keys:   keys,
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
	mux.HandleFunc("POST /sample/{id}", seedHandler)
	mux.HandleFunc("DELETE /pipeline/delete/{id}", deleteHandler)
	mux.HandleFunc("DELETE /pipeline/unlink/{id}", unlinkHandler)
	mux.HandleFunc("DELETE /sample", cleanupHandler)

	server := &http.Server{
		Addr:              httpAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("pipeline example listening on http://%s", httpAddr)
	log.Printf("redis address: %s", redisAddr)

	if err = server.ListenAndServe(); err != nil {
		log.Fatalln("unable to start web server:", err)
	}
}

func seedSample(ctx context.Context, id string) ([]string, error) {
	rawItems := sampleRawSetItems(id)
	if err := client.SetMany(ctx, rawItems); err != nil {
		return nil, err
	}

	structItems := sampleStructSetItems(id)
	if err := client.SetStructMany(ctx, structItems); err != nil {
		return nil, err
	}

	keys := make([]string, 0, len(rawItems)+len(structItems))
	keys = append(keys, setItemKeys(rawItems)...)
	keys = append(keys, setItemKeys(structItems)...)

	return keys, nil
}

func sampleRawSetItems(id string) []xredis.SetItem {
	return []xredis.SetItem{
		{
			Key:        messageKey(id),
			Value:      "hello from xredis pipeline example",
			Expiration: sampleTTL,
		},
		{
			Key:        counterKey(id),
			Value:      100,
			Expiration: sampleTTL,
		},
	}
}

func sampleStructSetItems(id string) []xredis.SetItem {
	items := []xredis.SetItem{
		{
			Key: profileKey(id),
			Value: userProfile{
				ID:        id,
				Name:      "user-" + id,
				Status:    "active",
				CreatedAt: time.Now().UTC(),
			},
			Expiration: sampleTTL,
		},
		{
			Key: settingsKey(id),
			Value: userSettings{
				Theme:         "dark",
				Notifications: true,
			},
			Expiration: sampleTTL,
		},
	}

	for i := 1; i <= 5; i++ {
		items = append(items, xredis.SetItem{
			Key: deleteKey(id, i),
			Value: sampleValue{
				ID:    id,
				Kind:  "delete",
				Index: i,
			},
			Expiration: sampleTTL,
		})
	}

	for i := 1; i <= 5; i++ {
		items = append(items, xredis.SetItem{
			Key: unlinkKey(id, i),
			Value: sampleValue{
				ID:    id,
				Kind:  "unlink",
				Index: i,
			},
			Expiration: sampleTTL,
		})
	}

	return items
}

func setItemKeys(items []xredis.SetItem) []string {
	keys := make([]string, 0, len(items))

	for _, item := range items {
		keys = append(keys, item.Key)
	}

	return keys
}

func cleanupKeys() []string {
	keys := make([]string, 0, len(sampleIDs)*14)

	for _, id := range sampleIDs {
		keys = append(keys, sampleKeys(id)...)
	}

	return keys
}

func sampleKeys(id string) []string {
	keys := []string{
		messageKey(id),
		counterKey(id),
		profileKey(id),
		settingsKey(id),
	}

	keys = append(keys, deleteKeys(id)...)
	keys = append(keys, unlinkKeys(id)...)

	return keys
}

func deleteKeys(id string) []string {
	keys := make([]string, 0, 5)

	for i := 1; i <= 5; i++ {
		keys = append(keys, deleteKey(id, i))
	}

	return keys
}

func unlinkKeys(id string) []string {
	keys := make([]string, 0, 5)

	for i := 1; i <= 5; i++ {
		keys = append(keys, unlinkKey(id, i))
	}

	return keys
}

func messageKey(id string) string {
	return samplePrefix + "message:" + id
}

func profileKey(id string) string {
	return samplePrefix + "profile:" + id
}

func settingsKey(id string) string {
	return samplePrefix + "settings:" + id
}

func counterKey(id string) string {
	return samplePrefix + "counter:" + id
}

func deleteKey(id string, n int) string {
	return fmt.Sprintf("%sdelete:%s:%d", samplePrefix, id, n)
}

func unlinkKey(id string, n int) string {
	return fmt.Sprintf("%sunlink:%s:%d", samplePrefix, id, n)
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
