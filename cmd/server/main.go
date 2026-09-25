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

	"github.com/jackc/pgx/v5"
	"github.com/nhdewitt/spectra/internal/database"
	"github.com/nhdewitt/spectra/internal/secret"
	"github.com/nhdewitt/spectra/internal/server"
	"github.com/nhdewitt/spectra/internal/setup"
	"github.com/nhdewitt/spectra/internal/telemetry"
	"github.com/nhdewitt/spectra/internal/version"
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
	shutdownTracing, tracing, err := telemetry.Setup(ctx, version.Version)
	if err != nil {
		log.Fatalf("Failed to set up tracing: %v", err)
	}

	// nil when tracing is off
	var dbTracer pgx.QueryTracer
	if tracing {
		dbTracer = telemetry.NewPgxTracer()
	}

	pool, err := database.NewPool(ctx, cfg.DatabaseURL, dbTracer)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Store wraps the generated queries with the pool, so the server can open a
	// transaction for a metric batch without holding a pool reference itself.
	store := server.NewStore(pool)

	srvCfg := server.Config{
		Port:           cfg.ListenPort,
		ReleasesDir:    "releases",
		MaxConnections: 1024,
		ExternalURL:    cfg.ExternalURL,
		TLSCert:        cfg.TLSCert,
		TLSKey:         cfg.TLSKey,
		TLSCA:          cfg.TLSCA,
		TrustedProxies: cfg.TrustedProxies,
		Tracing:        tracing,
	}

	srv := server.New(srvCfg, store)

	if applied, err := setup.RunMigrations(ctx, pool); err != nil {
		srv.Logger.Error("migration failed", "error", err)
		os.Exit(1)
	} else if applied > 0 {
		srv.Logger.Info("applied pending migrations", "count", applied)
	} else {
		srv.Logger.Debug("no pending migrations")
	}

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

	err = srv.Shutdown(shutdownCtx)

	if tErr := shutdownTracing(shutdownCtx); tErr != nil {
		srv.Logger.Warn("trace flush failed", "error", tErr)
	}

	if err != nil {
		srv.Logger.Error("server exited", "error", err)
		os.Exit(1)
	}
	srv.Logger.Info("server stopped cleanly")
}
