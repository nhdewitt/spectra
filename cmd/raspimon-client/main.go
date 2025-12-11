package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nhdewitt/raspimon/collector"
	"github.com/nhdewitt/raspimon/metrics"
	"github.com/nhdewitt/raspimon/sender"
)

const (
	mountUpdateInterval = 30 * time.Second
)

func main() {
	serverURL := os.Getenv("RASPIMON_SERVER")
	if serverURL == "" {
		serverURL = "http://127.0.0.1:8080/metrics"
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nReceived termination signal. Shutting down...")
		cancel()
	}()

	hostname, _ := os.Hostname()
	metricsCh := make(chan metrics.Envelope, 100)
	c := collector.New(hostname, metricsCh)
	s := sender.New(serverURL, metricsCh)

	driveCache := collector.NewDriveCache()
	go collector.RunMountManager(ctx, driveCache, mountUpdateInterval)
	time.Sleep(1 * time.Second)

	diskCollector := collector.MakeDiskCollector(driveCache)
	diskIOCollector := collector.MakeDiskIOCollector(driveCache)

	go s.Run(ctx)

	go c.Run(ctx, 5*time.Second, collector.CollectCPU)
	go c.Run(ctx, 10*time.Second, collector.CollectMemory)
	go c.Run(ctx, 60*time.Second, diskCollector)
	go c.Run(ctx, 5*time.Second, diskIOCollector)
	/*
		go c.Run(ctx, 5*time.Second, collectNetwork)
		go c.Run(ctx, 10*time.Second, collectTemperature)
		go c.Run(ctx, 15*time.Second, collectProcesses)
		go c.Run(ctx, 10*time.Second, collectThrottle)
		go c.Run(ctx, 15*time.Second, collectClock)
		go c.Run(ctx, 60*time.Second, collectVoltage)
		go c.Run(ctx, 30*time.Second, collectWiFi)
		go c.Run(ctx, 60*time.Second, collectGPU)
		go c.Run(ctx, 300*time.Second, collectSystem)
	*/

	<-ctx.Done()
	fmt.Println("Main application exiting.")
}
