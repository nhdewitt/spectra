package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"syscall"
)

func main() {
	store := NewAgentStore()
	mux := http.NewServeMux()

	mux.HandleFunc("/api/v1/metrics", handleMetrics)
	mux.HandleFunc("/api/v1/agent/command", handleAgentCommand(store))
	mux.HandleFunc("/api/v1/agent/logs", handleAgentLogs)
	mux.HandleFunc("/admin/trigger_logs", handleAdminTriggerLogs(store))

	listenAddr := "0.0.0.0:8080"
	log.Printf("Spectra Server starting on %s", listenAddr)

	lc := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			return nil
		},
	}

	listener, err := lc.Listen(context.Background(), "tcp4", listenAddr)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	defer listener.Close()

	if err := http.Serve(listener, mux); err != nil {
		log.Fatal(err)
	}
}
