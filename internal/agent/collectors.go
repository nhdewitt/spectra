package agent

import (
	"context"
	"log"
	"time"

	"github.com/nhdewitt/spectra/internal/collector"
	"github.com/nhdewitt/spectra/internal/collector/containers"
	"github.com/nhdewitt/spectra/internal/collector/cpu"
	"github.com/nhdewitt/spectra/internal/collector/disk"
	"github.com/nhdewitt/spectra/internal/collector/memory"
	"github.com/nhdewitt/spectra/internal/collector/network"
	"github.com/nhdewitt/spectra/internal/collector/pi"
	"github.com/nhdewitt/spectra/internal/collector/processes"
	"github.com/nhdewitt/spectra/internal/collector/services"
	"github.com/nhdewitt/spectra/internal/collector/system"
	"github.com/nhdewitt/spectra/internal/collector/temperature"
	"github.com/nhdewitt/spectra/internal/collector/wifi"
	"github.com/nhdewitt/spectra/internal/inventory"
	"github.com/nhdewitt/spectra/internal/protocol"
)

// job is a helper struct for internal use.
type job struct {
	Interval time.Duration
	Fn       collector.CollectFunc
}

func (a *Agent) startCollectors(ctx context.Context) {
	c := collector.New(a.Config.Hostname, a.metricsCh)

	diskCol := disk.MakeDiskCollector(a.DriveCache)
	diskIOCol := disk.MakeDiskIOCollector(a.DriveCache)
	svcCol := services.MakeCollector(a.Platform.SystemctlPath)
	tempCol := temperature.MakeCollector(a.Platform.ThermalZones)

	jobs := []job{
		{5 * time.Second, cpu.Collect},
		{10 * time.Second, memory.Collect},
		{5 * time.Second, network.Collect},
		{300 * time.Second, system.Collect},
		{60 * time.Second, diskCol},
		{5 * time.Second, diskIOCol},
		{60 * time.Second, svcCol},
		{15 * time.Second, processes.Collect},
		{10 * time.Second, tempCol},
		{30 * time.Second, wifi.Collect},
		{60 * time.Second, containers.Collect},
	}

	for _, j := range jobs {
		go c.Run(ctx, j.Interval, j.Fn)
	}

	if a.Platform.IsRaspberryPi {
		piJobs := []job{
			{15 * time.Second, pi.CollectClocks},
			{10 * time.Second, pi.CollectThrottle},
			{60 * time.Second, pi.CollectVoltage},
			{60 * time.Second, pi.CollectGPU},
		}
		for _, j := range piJobs {
			go c.Run(ctx, j.Interval, j.Fn)
		}
	}

	// Nightly tasks
	go a.runNightly(ctx, 2, 0, func() {
		apps, err := inventory.GetInstalledApps(ctx)
		if err != nil {
			log.Printf("nightly apps failed: %v", err)
			return
		}
		a.metricsCh <- protocol.Envelope{
			Type:      "application_list",
			Timestamp: time.Now(),
			Hostname:  a.Config.Hostname,
			Data:      &protocol.ApplicationListMetric{Applications: apps},
		}
	})

	go a.runNightly(ctx, 2, 5, func() {
		metrics, err := inventory.GetUpdates(ctx)
		if err != nil {
			log.Printf("nightly updates failed: %v", err)
			return
		}
		for _, m := range metrics {
			a.metricsCh <- protocol.Envelope{
				Type:      m.MetricType(),
				Timestamp: time.Now(),
				Hostname:  a.Config.Hostname,
				Data:      m,
			}
		}
	})
}
