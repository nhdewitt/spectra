package agent

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/nhdewitt/spectra/internal/collector/cpu"
	"github.com/nhdewitt/spectra/internal/collector/disk"
	"github.com/nhdewitt/spectra/internal/protocol"
)

func TestJob_Struct(t *testing.T) {
	j := job{
		Interval: 5 * time.Second,
		Fn:       cpu.Collect,
	}

	if j.Interval != 5*time.Second {
		t.Errorf("Interval: got %v, want 5s", j.Interval)
	}
	if j.Fn == nil {
		t.Error("Fn should not be nil")
	}
}

func TestStartCollectors_ContextCancelled(t *testing.T) {
	a := New(Config{Hostname: "test-agent", IdentityPath: filepath.Join(t.TempDir(), "agent-id.json")})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	a.startCollectors(ctx)
	time.Sleep(100 * time.Millisecond)
	cancel()

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

func TestSendEnvelope(t *testing.T) {
	a := New(Config{Hostname: "test-agent", IdentityPath: filepath.Join(t.TempDir(), "agent-id.json")})
	env := protocol.Envelope{Type: "application_list", Hostname: "test-agent"}

	a.sendEnvelope(context.Background(), env)
	if got := len(a.metricsCh); got != 1 {
		t.Fatalf("queued %d envelopes, want 1", got)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	a.sendEnvelope(ctx, env)
	if got := len(a.metricsCh); got != 1 {
		t.Errorf("queued %d envelopes after cancel, want 1", got)
	}
}

func TestSendEnvelope_FullChannelUnblocksOnCancel(t *testing.T) {
	a := New(Config{Hostname: "test-agent", IdentityPath: filepath.Join(t.TempDir(), "agent-id.json")})
	for range cap(a.metricsCh) {
		a.metricsCh <- protocol.Envelope{}
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		a.sendEnvelope(ctx, protocol.Envelope{Type: "application_list"})
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("sendEnvelope stayed blocked on a full channel after cancel")
	}
}

func TestMakeDiskCollector(t *testing.T) {
	cache := disk.NewDriveCache()
	diskCol := disk.MakeDiskCollector(cache)

	if diskCol == nil {
		t.Error("MakeDiskCollector returned nil")
	}
}

func TestMakeDiskIOCollector(t *testing.T) {
	cache := disk.NewDriveCache()
	diskIOCol := disk.MakeDiskIOCollector(cache)

	if diskIOCol == nil {
		t.Error("MakeDiskIOCollector returned nil")
	}
}

func BenchmarkMakeDiskCollector(b *testing.B) {
	cache := disk.NewDriveCache()

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = disk.MakeDiskCollector(cache)
	}
}

func BenchmarkMakeDiskIOCollector(b *testing.B) {
	cache := disk.NewDriveCache()

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = disk.MakeDiskIOCollector(cache)
	}
}
