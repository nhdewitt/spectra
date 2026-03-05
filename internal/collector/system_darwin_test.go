//go:build darwin

package collector

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func TestCollectSystem_Integration(t *testing.T) {
	ctx := context.Background()
	metrics, err := CollectSystem(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(metrics))
	}

	sm, ok := metrics[0].(protocol.SystemMetric)
	if !ok {
		t.Fatalf("expected SystemMetric, got %T", metrics[0])
	}

	if sm.Uptime == 0 {
		t.Error("uptime is 0")
	}
	if sm.BootTime == 0 {
		t.Error("bootTime is 0")
	}
	if sm.Processes < 10 {
		t.Errorf("process count = %d, expected at least 10", sm.Processes)
	}

	t.Logf("uptime=%ds bootTime=%d processes=%d users=%d", sm.Uptime, sm.BootTime, sm.Processes, sm.Users)
}

func TestGetBootTimeAndUptime(t *testing.T) {
	bootTime, uptime, err := getBootTimeAndUptime()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	now := uint64(time.Now().Unix())

	if bootTime == 0 {
		t.Error("bootTime is 0")
	}
	if bootTime > now {
		t.Errorf("bootTime %d is in the future (now=%d)", bootTime, now)
	}
	if uptime == 0 {
		t.Error("uptime is 0")
	}

	expected := now - bootTime
	if uptime > expected+2 || uptime < expected-2 {
		t.Errorf("uptime %d doesn't match now-bootTime %d (tolerance=2s)", uptime, expected)
	}

	t.Logf("bootTime=%d uptime=%ds (%.1f hours)", bootTime, uptime, float64(uptime)/3600)
}

func TestCountProcesses(t *testing.T) {
	ctx := context.Background()
	count, err := countProcesses(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count < 10 {
		t.Errorf("process count = %d, expected at least 10", count)
	}

	t.Logf("process count: %d", count)
}

func TestParseWhoFrom_MultipleUsers(t *testing.T) {
	input := "user1\tconsole\tFeb 25 10:00\nuser1\tttys000\tFeb 25 10:01\n"
	count := parseWhoFrom(strings.NewReader(input))
	if count != 2 {
		t.Errorf("got %d, want 2", count)
	}
}

func TestParseWhoFrom_Empty(t *testing.T) {
	count := parseWhoFrom(bytes.NewReader(nil))
	if count != 0 {
		t.Errorf("got %d, want 0", count)
	}
}

func TestParseWhoFrom_WhitespaceOnly(t *testing.T) {
	count := parseWhoFrom(strings.NewReader("  \n  \n"))
	if count != 0 {
		t.Errorf("got %d, want 0", count)
	}
}
