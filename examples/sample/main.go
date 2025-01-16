package main

import (
	"log"
	"net/http"
	"os"

	"github.com/mkbeh/xredis"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var client *redis.Client

var (
	addrs string
)

func init() {
	addrs = os.Getenv("REDIS_ADDRS")
}

func getTasksHandler(w http.ResponseWriter, req *http.Request) {
	var value string
	if err := client.Get(req.Context(), "value_1", &value); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(value))
}

func createTasksHandler(w http.ResponseWriter, req *http.Request) {
	if err := client.Set(req.Context(), "value_1", "First value", 0); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := client.Set(req.Context(), "value_2", "Second value", 0); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("OK"))
}

func main() {
	var err error

	cfg := &redis.Config{Addrs: addrs}

	client, err = redis.NewClient(
		redis.WithConfig(cfg),
		redis.WithClientID("test-client"),
	)
	if err != nil {
		log.Fatalln(err)
	}

	defer client.Close()

	http.HandleFunc("/get", getTasksHandler)
	http.HandleFunc("/create", createTasksHandler)
	http.Handle("/metrics", promhttp.Handler())

	err = http.ListenAndServe("localhost:8080", nil)
	if err != nil {
		log.Fatalln("Unable to start web server:", err)
	}
}
