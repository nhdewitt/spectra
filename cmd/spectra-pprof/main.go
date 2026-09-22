package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nhdewitt/spectra/internal/pprofd"
	"github.com/nhdewitt/spectra/internal/version"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:9091", "Listen address")
	dir := flag.String("dir", "/var/lib/spectra-pprof", "Directory to store profiles in")
	retain := flag.Duration("retain", 14*24*time.Hour, "Delete profiles older than this (0 to keep forever)")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *showVersion {
		log.Printf("spectra-pprof %s", version.Full())
		return
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	store, err := pprofd.NewStore(*dir, *retain)
	if err != nil {
		log.Fatalf("Failed to open profile store: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go runPruner(ctx, store, logger, *retain)

	srv := &http.Server{
		Addr:              *addr,
		Handler:           pprofd.NewServer(store, logger),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		logger.Info("spectra-pprof listening", "addr", *addr, "dir", *dir, "retain", *retain)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown failed", "error", err)
	}
}

func runPruner(ctx context.Context, store *pprofd.Store, logger *slog.Logger, retain time.Duration) {
	if retain <= 0 {
		return
	}

	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for {
		if n, err := store.Prune(time.Now()); err != nil {
			logger.Warn("prune failed", "error", err)
		} else if n > 0 {
			logger.Info("pruned old profiles", "count", n)
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
