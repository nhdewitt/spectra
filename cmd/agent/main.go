package main

import (
	"flag"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"

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
			if err := http.ListenAndServe("127.0.0.1:6060", nil); err != nil {
				log.Printf("failed to start debug server: %v", err)
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
