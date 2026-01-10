package main

import (
	"log"
	"net/http"
	"time"
)

func main() {
	store := NewAgentStore()
	mux := http.NewServeMux()

	// Routes
	mux.HandleFunc("/api/v1/metrics", handleMetrics)
	mux.HandleFunc("/api/v1/agent/command", handleAgentCommand(store))
	mux.HandleFunc("/api/v1/agent/command_result", handleCommandResult)
	mux.HandleFunc("/admin/trigger_logs", handleAdminTriggerLogs(store))
	mux.HandleFunc("/admin/trigger_disk", handleAdminTriggerDisk(store))
	mux.HandleFunc("/admin/trigger_network", handleAdminTriggerNetwork(store))

	server := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 40 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	log.Println("Server listening on :8080...")

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", err)
	}
}
