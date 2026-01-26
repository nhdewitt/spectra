package agent

import (
	"time"

	"github.com/nhdewitt/spectra/internal/collector"
)

// job is a helper struct for internal use.
type job struct {
	Interval time.Duration
	Fn       collector.CollectFunc
}

func (a *Agent) startCollectors() {
	c := collector.New(a.Config.Hostname, a.metricsCh)

	diskCol := collector.MakeDiskCollector(a.DriveCache)
	diskIOCol := collector.MakeDiskIOCollector(a.DriveCache)

	jobs := []job{
		{5 * time.Second, collector.CollectCPU},
		{10 * time.Second, collector.CollectMemory},
		{5 * time.Second, collector.CollectNetwork},
		{300 * time.Second, collector.CollectSystem},
		{60 * time.Second, diskCol},
		{5 * time.Second, diskIOCol},
		{60 * time.Second, collector.CollectServices},
		{15 * time.Second, collector.CollectProcesses},
		{10 * time.Second, collector.CollectTemperature},
		{30 * time.Second, collector.CollectWiFi},
		{15 * time.Second, collector.CollectPiClocks},
		{10 * time.Second, collector.CollectPiThrottle},
		{60 * time.Second, collector.CollectPiVoltage},
		{60 * time.Second, collector.CollectPiGPU},
		{60 * time.Second, collector.CollectContainers},
	}

	for _, j := range jobs {
		go c.Run(a.ctx, j.Interval, j.Fn)
	}

	// Nightly tasks
	go a.runNightly(2, 0, collector.GetInstalledApps)
}
