//go:build windows

package collector

import (
	"context"
	"os"
	"testing"
	"time"
	"unsafe"

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

func TestCollectProcesses_Baseline(t *testing.T) {
	lastWinProcessStates = make(map[uint32]winProcessState)

	ctx := context.Background()
	data, err := CollectProcesses(ctx)
	if err != nil {
		t.Fatalf("CollectProcesses failed: %v", err)
	}

	listMetric := data[0].(protocol.ProcessListMetric)

	// First call - all CPU should be 0
	for _, p := range listMetric.Processes {
		if p.CPUPercent != 0 {
			t.Errorf("PID %d has non-zero CPU on baseline: %f", p.Pid, p.CPUPercent)
		}
	}
}

func TestCollectProcesses_MemPercentRange(t *testing.T) {
	lastWinProcessStates = make(map[uint32]winProcessState)

	ctx := context.Background()
	data, err := CollectProcesses(ctx)
	if err != nil {
		t.Fatalf("CollectProcesses failed: %v", err)
	}

	listMetric := data[0].(protocol.ProcessListMetric)

	for _, p := range listMetric.Processes {
		if p.MemPercent < 0 || p.MemPercent > 100 {
			t.Errorf("PID %d (%s) has invalid MemPercent: %f", p.Pid, p.Name, p.MemPercent)
		}
	}
}

func TestCollectProcesses_StateCleanup(t *testing.T) {
	lastWinProcessStates = map[uint32]winProcessState{
		99999999: {LastTime: time.Now(), LastKernel: 100, LastUser: 100},
	}

	ctx := context.Background()
	_, err := CollectProcesses(ctx)
	if err != nil {
		t.Fatalf("CollectProcesses failed: %v", err)
	}

	if _, ok := lastWinProcessStates[99999999]; ok {
		t.Error("old fake PID not cleaned up")
	}
}

func TestGetProcessStatus_Integration(t *testing.T) {
	states, err := getProcessStatus()
	if err != nil {
		t.Fatalf("getProcessStatus failed: %v", err)
	}

	if len(states) == 0 {
		t.Fatal("No process states returned")
	}

	t.Logf("Got status for %d processes", len(states))

	runningCount := 0
	waitingCount := 0

	for pid, status := range states {
		switch status {
		case "Running":
			runningCount++
		case "Waiting":
			waitingCount++
		default:
			t.Errorf("PID %d has unexpected status: %s", pid, status)
		}
	}

	t.Logf("Running: %d, Waiting: %d", runningCount, waitingCount)

	if waitingCount == 0 {
		t.Log("Warning: no waiting processes found")
	}
}

func TestGetProcessStatus_ContainsSelf(t *testing.T) {
	states, err := getProcessStatus()
	if err != nil {
		t.Fatalf("getProcessStatus failed: %v", err)
	}

	myPid := uint32(os.Getpid())
	status, ok := states[myPid]
	if !ok {
		t.Errorf("own PID %d not found in states", myPid)
		return
	}

	t.Logf("Self (PID %d) status: %s", myPid, status)
}

func TestGetProcessStatus_SystemProcesses(t *testing.T) {
	states, err := getProcessStatus()
	if err != nil {
		t.Fatalf("getProcessStatus failed: %v", err)
	}

	if status, ok := states[0]; ok {
		t.Logf("PID 0 status: %s", status)
	} else {
		t.Log("PID 0 not in states")
	}

	if status, ok := states[4]; ok {
		t.Logf("PID 4 (System) status: %s", status)
	} else {
		t.Log("PID 4 not in states")
	}
}

func TestSystemThreadInformation_Size(t *testing.T) {
	size := unsafe.Sizeof(systemThreadInformation{})

	if unsafe.Sizeof(uintptr(0)) == 8 {
		if size != 80 {
			t.Errorf("[64-bit] systemThreadInformation size = %d, want 80", size)
		}
	} else {
		if size != 64 {
			t.Errorf("[32-bit] systemThreadInformation size = %d, want 64", size)
		}
	}

	t.Logf("systemThreadInformation size: %d bytes", size)
}

func TestSystemProcessInformation_Size(t *testing.T) {
	size := unsafe.Sizeof(systemProcessInformation{})

	t.Logf("systemProcessInformation size: %d bytes", size)
	if size < 100 {
		t.Errorf("systemProcessInformation seems too small: %d bytes", size)
	}
}

func TestProcessState_Constants(t *testing.T) {
	tests := []struct {
		name  string
		state ProcessState
		value uint32
	}{
		{"StateInitialized", StateInitialized, 0},
		{"StateReady", StateReady, 1},
		{"StateRunning", StateRunning, 2},
		{"StateStandby", StateStandby, 3},
		{"StateTerminated", StateTerminated, 4},
		{"StateWaiting", StateWaiting, 5},
	}

	for _, tt := range tests {
		if uint32(tt.state) != tt.value {
			t.Errorf("%s = %d, want %d", tt.name, tt.state, tt.value)
		}
	}
}

func BenchmarkCollectProcesses(b *testing.B) {
	lastWinProcessStates = make(map[uint32]winProcessState)
	ctx := context.Background()

	// Prime
	_, _ = CollectProcesses(ctx)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = CollectProcesses(ctx)
	}
}

func BenchmarkGetProcessStatus(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_, _ = getProcessStatus()
	}
}

func BenchmarkCollectProcesses_WithStatus(b *testing.B) {
	lastWinProcessStates = make(map[uint32]winProcessState)
	ctx := context.Background()

	// Prime
	_, _ = CollectProcesses(ctx)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = CollectProcesses(ctx)
		_, _ = getProcessStatus()
	}
}
