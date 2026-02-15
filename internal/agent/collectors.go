package agent

import (
	"log"
	"time"

	"github.com/nhdewitt/spectra/internal/collector"
	"github.com/nhdewitt/spectra/internal/protocol"
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
	tempCol := collector.MakeTemperatureCollector(a.Platform.ThermalZones)

	jobs := []job{
		{5 * time.Second, collector.CollectCPU},
		{10 * time.Second, collector.CollectMemory},
		{5 * time.Second, collector.CollectNetwork},
		{300 * time.Second, collector.CollectSystem},
		{60 * time.Second, diskCol},
		{5 * time.Second, diskIOCol},
		{60 * time.Second, collector.CollectServices},
		{15 * time.Second, collector.CollectProcesses},
		{10 * time.Second, tempCol},
		{30 * time.Second, collector.CollectWiFi},
		{60 * time.Second, collector.CollectPiGPU},
		{60 * time.Second, collector.CollectContainers},
	}

	for _, j := range jobs {
		go c.Run(a.ctx, j.Interval, j.Fn)
	}

	if a.Platform.IsRaspberryPi {
		piJobs := []job{
			{15 * time.Second, collector.CollectPiClocks},
			{10 * time.Second, collector.CollectPiThrottle},
			{60 * time.Second, collector.CollectPiVoltage},
			{60 * time.Second, collector.CollectPiGPU},
		}
		for _, j := range piJobs {
			go c.Run(a.ctx, j.Interval, j.Fn)
		}
	}

	// Nightly tasks
	go a.runNightly(2, 0, func() {
		apps, err := collector.GetInstalledApps(a.ctx)
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

	go a.runNightly(2, 5, func() {
		metrics, err := collector.CollectUpdates(a.ctx)
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
