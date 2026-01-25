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
			if proc.ThreadsTotal == 0 {
				t.Error("Test runner reported 0 threads")
			}
		}
	}

	if !foundSelf {
		t.Error("Could not find own PID in process list")
	}
}

func TestCollectProcesses_StatusDistribution(t *testing.T) {
	lastWinProcessStates = make(map[uint32]winProcessState)

	ctx := context.Background()
	data, err := CollectProcesses(ctx)
	if err != nil {
		t.Fatalf("CollectProcesses failed: %v", err)
	}

	listMetric := data[0].(protocol.ProcessListMetric)

	statusCounts := make(map[protocol.ProcStatus]int)
	for _, p := range listMetric.Processes {
		statusCounts[p.Status]++
	}

	t.Logf("Status distribution:")
	t.Logf("  Running:	%d", statusCounts[protocol.ProcRunning])
	t.Logf("  Runnable:	%d", statusCounts[protocol.ProcRunnable])
	t.Logf("  Waiting:	%d", statusCounts[protocol.ProcWaiting])
	t.Logf("  Other:	%d", statusCounts[protocol.ProcOther])

	if statusCounts[protocol.ProcWaiting] == 0 {
		t.Error("No waiting processes found - expected most to be waiting")
	}

	// At least the test runner should be running
	totalActive := statusCounts[protocol.ProcRunning] + statusCounts[protocol.ProcRunnable]
	if totalActive == 0 {
		t.Error("No running or runnable processes found")
	}
}

func TestCollectProcesses_ThreadCounts(t *testing.T) {
	lastWinProcessStates = make(map[uint32]winProcessState)

	ctx := context.Background()
	data, err := CollectProcesses(ctx)
	if err != nil {
		t.Fatalf("CollectProcesses failed: %v", err)
	}

	listMetric := data[0].(protocol.ProcessListMetric)

	for _, p := range listMetric.Processes {
		if p.Pid == 0 || p.Pid == 4 {
			continue
		}

		if p.ThreadsRunning != nil && p.ThreadsRunnable != nil && p.ThreadsWaiting != nil {
			sum := *p.ThreadsRunning + *p.ThreadsRunnable + *p.ThreadsWaiting
			if sum > p.ThreadsTotal {
				t.Errorf("PID %d: thread sum (%d) > total (%d)", p.Pid, sum, p.ThreadsTotal)
			}
		}

		if p.ThreadsTotal > 10000 {
			t.Errorf("PID %d has suspicious thread count: %d", p.Pid, p.ThreadsTotal)
		}
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

func TestCollectProcesses_ValidStatus(t *testing.T) {
	lastWinProcessStates = make(map[uint32]winProcessState)

	ctx := context.Background()
	data, err := CollectProcesses(ctx)
	if err != nil {
		t.Fatalf("CollectProcesses failed: %v", err)
	}

	listMetric := data[0].(protocol.ProcessListMetric)

	validStatuses := map[protocol.ProcStatus]bool{
		protocol.ProcRunning:  true,
		protocol.ProcRunnable: true,
		protocol.ProcWaiting:  true,
		protocol.ProcOther:    true,
	}

	for _, p := range listMetric.Processes {
		if !validStatuses[p.Status] {
			t.Errorf("PID %d has invalid status: %s", p.Pid, p.Status)
		}
	}
}

func TestGetProcessSchedulerSummary_Integration(t *testing.T) {
	sched, err := getProcessSchedulerSummary()
	if err != nil {
		t.Fatalf("getProcessSchedulerSummary failed: %v", err)
	}

	if len(sched) == 0 {
		t.Fatal("No scheduler summaries returned")
	}

	t.Logf("Got scheduler summary for %d processes", len(sched))

	statusCounts := make(map[protocol.ProcStatus]int)
	var totalThreads uint32

	for _, s := range sched {
		statusCounts[s.Status]++
		totalThreads += s.ThreadsTotal
	}

	t.Logf("Process status distribution:")
	t.Logf("  Running:  %d", statusCounts[protocol.ProcRunning])
	t.Logf("  Runnable: %d", statusCounts[protocol.ProcRunnable])
	t.Logf("  Waiting:  %d", statusCounts[protocol.ProcWaiting])
	t.Logf("  Other:    %d", statusCounts[protocol.ProcOther])
	t.Logf("Total threads across all processes: %d", totalThreads)

	if statusCounts[protocol.ProcWaiting] == 0 {
		t.Error("No waiting processes found")
	}
}

func TestGetProcessSchedulerSummary_ContainsSelf(t *testing.T) {
	sched, err := getProcessSchedulerSummary()
	if err != nil {
		t.Fatalf("getProcessSchedulerSummary failed: %v", err)
	}

	myPid := uint32(os.Getpid())
	s, ok := sched[myPid]
	if !ok {
		t.Errorf("own PID %d not found in scheduler summary", myPid)
		return
	}

	t.Logf("Self (PID %d): Status=%s Threads=%d (Running=%d, Runnable=%d, Waiting=%d)",
		myPid, s.Status, s.ThreadsTotal, s.ThreadsRunning, s.ThreadsRunnable, s.ThreadsWaiting)

	if s.ThreadsTotal == 0 {
		t.Error("own process reported 0 threads")
	}
}

func TestGetProcessSchedulerSummary_ThreadStateConsistency(t *testing.T) {
	sched, err := getProcessSchedulerSummary()
	if err != nil {
		t.Fatalf("getProcessSchedulerSummary failed: %v", err)
	}

	for pid, s := range sched {
		// Running + Runnable + Waiting should be <= Total
		sum := s.ThreadsRunning + s.ThreadsRunnable + s.ThreadsWaiting
		if sum > s.ThreadsTotal {
			t.Errorf("PID %d: thread state sum (%d) > total (%d)", pid, sum, s.ThreadsTotal)
		}

		switch s.Status {
		case protocol.ProcRunning:
			if s.ThreadsRunning == 0 {
				t.Errorf("PID %d: status=running but ThreadsRunning=0", pid)
			}
		case protocol.ProcRunnable:
			if s.ThreadsRunnable == 0 {
				t.Errorf("PID %d: status=runnable but ThreadsRunnable=0", pid)
			}
		case protocol.ProcWaiting:
			if s.ThreadsWaiting == 0 {
				t.Errorf("PID %d: status=waiting but ThreadsWaiting=0", pid)
			}
		}
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
		{"StateTransition", StateTransition, 6},
		{"StateDeferredReady", StateDeferredReady, 7},
		{"StateGateWaitObsolete", StateGateWaitObsolete, 8},
		{"StateWaitingForProcessInSwap", StateWaitingForProcessInSwap, 9},
	}

	for _, tt := range tests {
		if uint32(tt.state) != tt.value {
			t.Errorf("%s = %d, want %d", tt.name, tt.state, tt.value)
		}
	}
}

func TestProcStatus_Values(t *testing.T) {
	tests := []struct {
		status protocol.ProcStatus
		want   string
	}{
		{protocol.ProcRunning, "running"},
		{protocol.ProcRunnable, "runnable"},
		{protocol.ProcWaiting, "waiting"},
		{protocol.ProcOther, "other"},
	}

	for _, tt := range tests {
		if string(tt.status) != tt.want {
			t.Errorf("ProcStatus %v = %q, want %q", tt.status, string(tt.status), tt.want)
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

func BenchmarkGetProcessSchedulerSummary(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_, _ = getProcessSchedulerSummary()
	}
}

func BenchmarkCollectProcesses_NoScheduler(b *testing.B) {
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
