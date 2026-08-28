package agent

import (
	"context"
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
	Name     string
	Interval time.Duration
	Fn       collector.CollectFunc
}

func (a *Agent) startCollectors(ctx context.Context) {
	c := collector.New(a.Config.Hostname, a.metricsCh, a.Logger.Logger)

	diskCol := disk.MakeDiskCollector(a.DriveCache)
	diskIOCol := disk.MakeDiskIOCollector(a.DriveCache)
	svcCol := services.MakeCollector(a.Platform.SystemctlPath)
	tempCol := temperature.MakeCollector(a.Platform.ThermalZones)

	jobs := []job{
		{"cpu", 5 * time.Second, cpu.Collect},
		{"memory", 10 * time.Second, memory.Collect},
		{"network", 5 * time.Second, network.Collect},
		{"system", 300 * time.Second, system.Collect},
		{"disk", 60 * time.Second, diskCol},
		{"disk_io", 5 * time.Second, diskIOCol},
		{"services", 60 * time.Second, svcCol},
		{"processes", 15 * time.Second, processes.Collect},
		{"temperature", 10 * time.Second, tempCol},
		{"wifi", 30 * time.Second, wifi.Collect},
		{"containers", 60 * time.Second, containers.Collect},
	}

	for _, j := range jobs {
		go c.Run(ctx, j.Name, j.Interval, j.Fn)
	}

	if a.Platform.IsRaspberryPi {
		piJobs := []job{
			{"pi_clocks", 15 * time.Second, pi.CollectClocks},
			{"pi_throttle", 10 * time.Second, pi.CollectThrottle},
			{"pi_voltage", 60 * time.Second, pi.CollectVoltage},
			{"pi_gpu", 60 * time.Second, pi.CollectGPU},
		}
		for _, j := range piJobs {
			go c.Run(ctx, j.Name, j.Interval, j.Fn)
		}
	}

	// Nightly tasks
	go a.runNightly(ctx, 2, 0, func() {
		apps, err := inventory.GetInstalledApps(ctx)
		if err != nil {
			a.Logger.Warn("nightly apps collection failed", "error", err)
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
			a.Logger.Warn("nightly updates collection failed", "error", err)
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
