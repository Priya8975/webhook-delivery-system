package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync/atomic"
	"time"
)

var requestCount atomic.Int64

func main() {
	port := "9090"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}

	// Successful endpoint — always returns 200
	http.HandleFunc("/webhook/success", func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		logRequest(r, count, 200)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "received"})
	})

	// Slow endpoint — delays 3 seconds before responding
	http.HandleFunc("/webhook/slow", func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		time.Sleep(3 * time.Second)
		logRequest(r, count, 200)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "received (slow)"})
	})

	// Failing endpoint — always returns 500
	http.HandleFunc("/webhook/fail", func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		logRequest(r, count, 500)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
	})

	// Stats endpoint — shows request count
	http.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]int64{"total_requests": requestCount.Load()})
	})

	log.Printf("Mock endpoint server starting on :%s", port)
	log.Printf("  POST /webhook/success  -> 200 OK")
	log.Printf("  POST /webhook/slow     -> 200 OK (3s delay)")
	log.Printf("  POST /webhook/fail     -> 500 Error")
	log.Printf("  GET  /stats            -> request count")

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func logRequest(r *http.Request, count int64, status int) {
	fmt.Printf("[#%d] %s %s -> %d | sig=%s event=%s id=%s attempt=%s\n",
		count,
		r.Method,
		r.URL.Path,
		status,
		truncate(r.Header.Get("X-Webhook-Signature"), 16),
		r.Header.Get("X-Webhook-Event"),
		truncate(r.Header.Get("X-Webhook-ID"), 8),
		r.Header.Get("X-Webhook-Attempt"),
	)
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
