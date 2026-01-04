package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/nhdewitt/spectra/internal/collector"
	"github.com/nhdewitt/spectra/internal/protocol"
	"github.com/nhdewitt/spectra/internal/sender"
)

const (
	mountUpdateInterval = 30 * time.Second
)

// Config holds all URL paths and identity info
type Config struct {
	BaseURL     string
	Hostname    string
	MetricsPath string
	CommandPath string
	LogsPath    string
}

func main() {
	cfg := loadConfig()
	fmt.Printf("Spectra Agent starting on %s...\n", cfg.Hostname)
	fmt.Printf("Server: %s\n", cfg.BaseURL)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	setupSignalHandler(cancel)

	// Channels & Caches
	metricsCh := make(chan protocol.Envelope, 100)
	driveCache := collector.NewDriveCache()

	// Subsystems
	// Mount Manager
	go collector.RunMountManager(ctx, driveCache, 30*time.Second)
	time.Sleep(1 * time.Second) // Warmup

	// Metrics Sender
	metricsSender := sender.New(cfg.BaseURL+cfg.MetricsPath, metricsCh)
	go metricsSender.Run(ctx)

	// Metric Collectors
	startCollectors(ctx, cfg.Hostname, metricsCh, driveCache)

	// Command Loop
	go runCommandLoop(ctx, cfg)

	// Block until shutdown
	<-ctx.Done()
	fmt.Println("Main application exiting.")
	time.Sleep(1 * time.Second)
}

func loadConfig() Config {
	base := os.Getenv("SPECTRA_SERVER")
	if base == "" {
		base = "http://127.0.0.1:8080"
	}
	base = strings.TrimSuffix(base, "/")
	host, _ := os.Hostname()

	return Config{
		BaseURL:     base,
		Hostname:    host,
		MetricsPath: "/api/v1/metrics",
		CommandPath: "/api/v1/agent/command",
		LogsPath:    "/api/v1/agent/logs",
	}
}

func setupSignalHandler(cancel context.CancelFunc) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nReceived termination signal. Shutting down...")
		cancel()
	}()
}

func startCollectors(ctx context.Context, hostname string, ch chan protocol.Envelope, cache *collector.DriveCache) {
	c := collector.New(hostname, ch)

	diskCol := collector.MakeDiskCollector(cache)
	diskIOCol := collector.MakeDiskIOCollector(cache)

	jobs := []struct {
		Interval time.Duration
		Fn       collector.CollectFunc
	}{
		{5 * time.Second, collector.CollectCPU},
		{10 * time.Second, collector.CollectMemory},
		{5 * time.Second, collector.CollectNetwork},
		{300 * time.Second, collector.CollectSystem},
		{60 * time.Second, diskCol},
		{5 * time.Second, diskIOCol},
		{15 * time.Second, collector.CollectProcesses},
		{10 * time.Second, collector.CollectTemperature},
		{30 * time.Second, collector.CollectWiFi},
		{15 * time.Second, collector.CollectPiClocks},
		{10 * time.Second, collector.CollectPiThrottle},
		{60 * time.Second, collector.CollectPiVoltage},
		{60 * time.Second, collector.CollectPiGPU},
	}

	for _, job := range jobs {
		go c.Run(ctx, job.Interval, job.Fn)
	}
}
