package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nhdewitt/spectra/internal/agent"
)

func main() {
	baseURL := os.Getenv("SPECTRA_SERVER")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8080"
	}

	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalf("Error getting hostname: %v", err)
	}
	if h := os.Getenv("HOSTNAME"); h != "" {
		hostname = h
	}

	cfg := agent.Config{
		BaseURL:      baseURL,
		Hostname:     hostname,
		MetricsPath:  "/api/v1/metrics",
		CommandPath:  "/api/v1/agent/command",
		PollInterval: 5 * time.Second,
	}

	a := agent.New(cfg)

	// Handle shutdown signals
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh

		log.Println("\nReceived termination signal...")
		a.Shutdown()
	}()

	if err := a.Start(); err != nil {
		log.Fatalf("Agent exited with error: %v", err)
	}
}
