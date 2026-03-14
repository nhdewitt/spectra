package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nhdewitt/spectra/internal/database"
	"github.com/nhdewitt/spectra/internal/server"
	"github.com/nhdewitt/spectra/internal/setup"
)

func main() {
	configPath := flag.String("config", setup.DefaultConfigPath, "Path to server config file")
	flag.Parse()

	if !setup.ConfigExists(*configPath) {
		log.Fatalf("No config found at %s - run spectra-setup first", *configPath)
	}

	cfg, err := setup.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	ctx := context.Background()
	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	queries := database.New(pool)

	srvCfg := server.Config{
		Port:        cfg.ListenPort,
		ReleasesDir: "releases",
	}

	srv := server.New(srvCfg, queries)

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		log.Printf("Received %v, shutting down...", sig)
	case err := <-errCh:
		log.Printf("Server error: %v", err)
		return
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Shutdown error: %v", err)
	}
	log.Println("Server stopped cleanly")
}
