//go:build freebsd

package collector

import (
	"bytes"
	"context"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func TestCollectSystem_Integration(t *testing.T) {
	metrics, err := CollectSystem(context.Background())
	if err != nil {
		t.Fatalf("CollectSystem: %v", err)
	}
	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(metrics))
	}

	sm, ok := metrics[0].(protocol.SystemMetric)
	if !ok {
		t.Fatalf("expected SystemMetric, got %T", metrics[0])
	}

	t.Logf("Uptime:		%d seconds (%.1f days)", sm.Uptime, float64(sm.Uptime)/86400)
	t.Logf("BootTime:	%d (unix epoch)", sm.BootTime)
	t.Logf("Processes:	%d", sm.Processes)
	t.Logf("Users:		%d", sm.Users)
}

func TestCollectSystem_BootTime(t *testing.T) {
	metrics, err := CollectSystem(context.Background())
	if err != nil {
		t.Fatalf("CollectSystem: %v", err)
	}
	sm := metrics[0].(protocol.SystemMetric)

	out, err := exec.Command("sysctl", "-n", "kern.boottime").Output()
	if err != nil {
		t.Fatalf("sysctl: %v", err)
	}

	s := strings.TrimSpace(string(out))
	idx := strings.Index(s, "sec = ")
	if idx == -1 {
		t.Fatalf("unexpected sysctl output: %q", s)
	}
	secStr := s[idx+6:]
	if comma := strings.Index(secStr, ","); comma != -1 {
		secStr = secStr[:comma]
	}
	sec, err := strconv.ParseUint(strings.TrimSpace(secStr), 10, 64)
	if err != nil {
		t.Fatalf("parse sec: %v (from %q)", err, s)
	}

	t.Logf("CollectSystem BootTime:	%d", sm.BootTime)
	t.Logf("sysctl kern.boottime:	%d", sec)

	if sm.BootTime != sec {
		t.Errorf("BootTime mismatch: collector=%d, sysctl=%d", sm.BootTime, sec)
	}
}

func TestCollectSystem_ProcessCount(t *testing.T) {
	metrics, err := CollectSystem(context.Background())
	if err != nil {
		t.Fatalf("CollectSystem: %v", err)
	}
	sm := metrics[0].(protocol.SystemMetric)

	out, err := exec.Command("ps", "-ax").Output()
	if err != nil {
		t.Fatalf("ps: %v", err)
	}
	numProcs := bytes.Count(out, []byte{'\n'}) - 1 // subtract header

	t.Logf("CollectSystem processes:		%d", sm.Processes)
	t.Logf("ps aux count:			%d", numProcs)

	// allow for some tolerance
	diff := sm.Processes - int(numProcs)
	if diff < 0 {
		diff = -diff
	}
	if diff > 5 {
		t.Errorf("process count mismatch: collector=%d, ps=%d (diff %d)", sm.Processes, numProcs, diff)
	}
}

func TestCollectSystem_UserCount(t *testing.T) {
	metrics, err := CollectSystem(context.Background())
	if err != nil {
		t.Fatalf("CollectSystem: %v", err)
	}
	sm := metrics[0].(protocol.SystemMetric)

	out, err := exec.Command("w").Output()
	if err != nil {
		t.Fatalf("who: %v", err)
	}

	numUsers := bytes.Count(out, []byte{'\n'}) - 2 // subtract header

	t.Logf("CollectSystem users:	%d", sm.Users)
	t.Logf("w count:		%d", numUsers)

	if sm.Users != numUsers {
		t.Errorf("user count mismatch: collector=%d, w=%d", sm.Users, numUsers)
	}
}

func BenchmarkCountProcs(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		countProcs()
	}
}
