package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("request received: %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
		fmt.Fprintf(w, "Hello from  test server! Time: %s\n", time.Now().Format(time.RFC3339))
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("health check from %s", r.RemoteAddr)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Periodic heartbeat log to show logs are streaming
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for t := range ticker.C {
			log.Printf("heartbeat: server alive at %s", t.Format(time.RFC3339))
		}
	}()

	log.Printf("test server starting on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}
