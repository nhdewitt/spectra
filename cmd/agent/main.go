package main

import (
	"errors"
	"flag"
	"log"
	"net/http"
	_ "net/http/pprof" // #nosec G108 -- debug-only pprof server is bound to 127.0.0.1
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nhdewitt/spectra/internal/agent"
)

func main() {
	// DEBUGGING
	debugMode := flag.Bool("debug", false, "Enable pprof debug server on localhost:6060")
	configPath := flag.String("config", "", "Path to agent config file (default: OS-specific)")
	flag.Parse()

	if *debugMode {
		go func() {
			log.Println("DEBUG MODE: pprof server running on http://127.0.0.1:6060/debug/pprof/")

			srv := &http.Server{
				Addr:              "127.0.0.1:6060",
				ReadHeaderTimeout: 5 * time.Second,
			}

			if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Printf("Failed to start debug server: %v", err)
			}
		}()
	}

	// Try loading config file
	path := *configPath
	if path == "" {
		path = agent.DefaultConfigPath()
	}

	cfg, err := agent.LoadConfig(path)
	if err != nil {
		log.Printf("No config file at %s, using environment variables", path)
		cfg = agent.ConfigFromEnv()
	}

	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalf("Error getting hostname: %v", err)
	}
	if h := os.Getenv("HOSTNAME"); h != "" {
		hostname = h
	}
	cfg.Hostname = hostname

	a := agent.New(*cfg)

	go func() {
		if err := a.Start(); err != nil {
			log.Fatalf("Agent exited with error: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	<-sigCh

	log.Println("\nReceived termination signal...")

	a.Shutdown()
}
