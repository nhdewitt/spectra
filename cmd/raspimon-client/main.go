package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nhdewitt/raspimon/collector"
	"github.com/nhdewitt/raspimon/metrics"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	hostname, _ := os.Hostname()

	metricsCh := make(chan metrics.Envelope, 100)

	c := collector.New(hostname, metricsCh)

	go c.Run(ctx, 5*time.Second, collector.CollectCPU)
	go c.Run(ctx, 10*time.Second, collector.CollectMemory)
	go c.Run(ctx, 60*time.Second, collector.CollectDisk)
	go c.Run(ctx, 60*time.Second, collectDiskIO)
	go c.Run(ctx, 5*time.Second, collectNetwork)
	go c.Run(ctx, 10*time.Second, collectTemperature)
	go c.Run(ctx, 15*time.Second, collectProcesses)
	go c.Run(ctx, 10*time.Second, collectThrottle)
	go c.Run(ctx, 15*time.Second, collectClock)
	go c.Run(ctx, 60*time.Second, collectVoltage)
	go c.Run(ctx, 30*time.Second, collectWiFi)
	go c.Run(ctx, 60*time.Second, collectGPU)
	go c.Run(ctx, 300*time.Second, collectSystem)
}
