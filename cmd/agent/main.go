package main

import (
	"flag"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nhdewitt/spectra/internal/agent"
)

func main() {
	// DEBUGGING
	debugMode := flag.Bool("debug", false, "Enable pprof debug server on localhost:6060")
	flag.Parse()

	if *debugMode {
		go func() {
			log.Println("DEBUG MODE: pprof server running on http://127.0.0.1:6060/debug/pprof/")
			if err := http.ListenAndServe("127.0.0.1:6060", nil); err != nil {
				log.Printf("failed to start debug server: %v", err)
			}
		}()
	}
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
