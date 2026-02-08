package agent

import (
	"testing"
	"time"

	"github.com/nhdewitt/spectra/internal/collector"
)

func TestJob_Struct(t *testing.T) {
	j := job{
		Interval: 5 * time.Second,
		Fn:       collector.CollectCPU,
	}

	if j.Interval != 5*time.Second {
		t.Errorf("Interval: got %v, want 5s", j.Interval)
	}
	if j.Fn == nil {
		t.Error("Fn should not be nil")
	}
}

func TestStartCollectors_DoesNotBlock(t *testing.T) {
	a := New(Config{Hostname: "test-agent"})

	done := make(chan any)
	go func() {
		a.startCollectors()
		close(done)
	}()

	select {
	case <-done:
		// Good - returned immediately
	case <-time.After(1 * time.Second):
		t.Error("startCollectors blocked")
	}

	a.cancel()
}

func TestStartCollectors_SpawnsGoroutines(t *testing.T) {
	a := New(Config{Hostname: "test-agent"})

	a.startCollectors()

	// Allow time for goroutines to start
	time.Sleep(100 * time.Millisecond)

	a.cancel()

	// Channel should have received some metrics
	count := 0
	for len(a.metricsCh) > 0 {
		<-a.metricsCh
		count++
	}

	t.Logf("Received %d metrics", count)
}

func TestStartCollectors_ContextCancelled(t *testing.T) {
	a := New(Config{Hostname: "test-agent"})

	a.startCollectors()
	time.Sleep(100 * time.Millisecond)
	a.cancel()

	var lastCount int
	for range 5 {
		time.Sleep(100 * time.Millisecond)
		for len(a.metricsCh) > 0 {
			<-a.metricsCh
			lastCount++
		}
	}

	finalCount := lastCount
	time.Sleep(200 * time.Millisecond)

	for len(a.metricsCh) > 0 {
		<-a.metricsCh
		lastCount++
	}

	if lastCount > finalCount {
		t.Errorf("metrics still arriving: %d new after stabilization", lastCount-finalCount)
	}
}

func TestMakeDiskCollector(t *testing.T) {
	cache := collector.NewDriveCache()
	diskCol := collector.MakeDiskCollector(cache)

	if diskCol == nil {
		t.Error("MakeDiskCollector returned nil")
	}
}

func TestMakeDiskIOCollector(t *testing.T) {
	cache := collector.NewDriveCache()
	diskIOCol := collector.MakeDiskIOCollector(cache)

	if diskIOCol == nil {
		t.Error("MakeDiskIOCollector returned nil")
	}
}

func BenchmarkJobSliceCreation(b *testing.B) {
	cache := collector.NewDriveCache()
	diskCol := collector.MakeDiskCollector(cache)
	diskIOCol := collector.MakeDiskIOCollector(cache)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
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
		_ = jobs
	}
}

func BenchmarkMakeDiskCollector(b *testing.B) {
	cache := collector.NewDriveCache()

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = collector.MakeDiskCollector(cache)
	}
}

func BenchmarkMakeDiskIOCollector(b *testing.B) {
	cache := collector.NewDriveCache()

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = collector.MakeDiskIOCollector(cache)
	}
}
