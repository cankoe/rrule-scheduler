package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync/atomic"
	"time"
)

type responseData struct {
	Datetime    string              `json:"datetime"`
	Method      string              `json:"method"`
	Path        string              `json:"path"`
	QueryParams map[string][]string `json:"query_params"`
	Headers     map[string][]string `json:"headers"`
}

var requestCounter uint64 // Atomic counter for requests

func handler(w http.ResponseWriter, r *http.Request) {
	// Increment request counter atomically
	count := atomic.AddUint64(&requestCounter, 1)

	// Log the count and user-agent
	log.Printf("Request #%d - User-Agent: %s\n", count, r.Header.Get("User-Agent"))

	// Prepare response JSON
	resp := responseData{
		Datetime:    time.Now().UTC().Format(time.RFC3339),
		Method:      r.Method,
		Path:        r.URL.Path,
		QueryParams: r.URL.Query(),
		Headers:     r.Header,
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Println("Error encoding response:", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func main() {
	http.HandleFunc("/", handler)
	log.Println("Server starting on :8081")
	if err := http.ListenAndServe(":8081", nil); err != nil {
		log.Fatal(err)
	}
}
