package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nhdewitt/spectra/internal/database"
	"github.com/nhdewitt/spectra/internal/secret"
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
		Port:           cfg.ListenPort,
		ReleasesDir:    "releases",
		MaxConnections: 1024,
		ExternalURL:    cfg.ExternalURL,
		TLSCert:        cfg.TLSCert,
		TLSKey:         cfg.TLSKey,
		TLSCA:          cfg.TLSCA,
	}

	srv := server.New(srvCfg, queries)

	cipher, err := secret.NewFromEnv()
	switch {
	case errors.Is(err, secret.ErrNoKey):
		srv.Logger.Warn("email delivery disabled: " + secret.KeyEnvVar + " not set")
	case err != nil:
		srv.Logger.Error("invalid secret key", "key", secret.KeyEnvVar, "error", err)
		os.Exit(1)
	default:
		srv.Cipher = cipher
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		srv.Logger.Info("received signal, shutting down", "signal", sig.String())
	case err := <-errCh:
		srv.Logger.Error("server error", "error", err)
		return
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		srv.Logger.Error("server exited", "error", err)
		os.Exit(1)
	}
	srv.Logger.Info("server stopped cleanly")
}
