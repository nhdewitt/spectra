//go:build windows

package collector

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func TestCollectProcesses_Integration(t *testing.T) {
	lastWinProcessStates = make(map[uint32]winProcessState)

	ctx := context.Background()

	_, err := CollectProcesses(ctx)
	if err != nil {
		t.Fatalf("First CollectProcesses failed: %v", err)
	}

	time.Sleep(1 * time.Second)

	data, err := CollectProcesses(ctx)
	if err != nil {
		t.Fatalf("Second CollectProcesses failed: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("Returned no metrics")
	}

	listMetric, ok := data[0].(protocol.ProcessListMetric)
	if !ok {
		t.Fatalf("Expected ProcessListMetric, got %T", data[0])
	}

	procs := listMetric.Processes
	t.Logf("Found %d processes running", len(procs))
	if len(procs) == 0 {
		t.Fatal("Process list is empty")
	}

	myPid := os.Getpid()
	foundSelf := false
	foundSystem := false

	for _, proc := range procs {
		if proc.MemRSS == 0 && proc.Pid != 0 && proc.Pid != 4 {
			t.Logf("PID %d (%s) reported 0 memory", proc.Pid, proc.Name)
		}
		if proc.Name == "" {
			t.Errorf("PID %d has empty name", proc.Pid)
		}

		if proc.Pid == myPid {
			foundSelf = true
			t.Logf("Found Test Runner (PID: %d): %s | Mem: %d bytes | CPU: %.2f%%", proc.Pid, proc.Name, proc.MemRSS, proc.CPUPercent)
			if proc.MemRSS == 0 {
				t.Error("Test runner reported 0 memory usage")
			}
		}

		if proc.Pid == 4 {
			foundSystem = true
		}
	}

	if !foundSelf {
		t.Error("Could not find own PID in process list")
	}
	if !foundSystem {
		t.Log("Could not find 'System' process (PID 4).")
	}
}

func BenchmarkCollectProcesses(b *testing.B) {
	ctx := context.Background()
	b.ReportAllocs()

	for b.Loop() {
		_, _ = CollectProcesses(ctx)
	}
}
