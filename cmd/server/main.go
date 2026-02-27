package main

import (
	"context"
	"flag"
	"log"

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
		Port: cfg.ListenPort,
	}

	srv := server.New(srvCfg, queries)

	if err := srv.Start(); err != nil {
		log.Fatalf("Server exited: %v", err)
	}
}
