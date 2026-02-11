//go:build freebsd

package collector

import (
	"bytes"
	"context"
	"encoding/binary"
	"strings"
	"testing"
	"unsafe"

	"github.com/nhdewitt/spectra/internal/protocol"
)

// TestKinfoProcSize verifies the struct's total binary.Read footprint
// matches the expected 600 bytes (through ki_numthreads).
func TestKinfoProcSize(t *testing.T) {
	got := int(unsafe.Sizeof(kinfoProc{}))
	// unsafe.Sizeof includes Go alignment padding, but we actually care
	// about what binary.Read consumes. Measure that directly.
	buf := make([]byte, 4096)
	r := bytes.NewReader(buf)
	var kp kinfoProc
	if err := binary.Read(r, binary.LittleEndian, &kp); err != nil {
		t.Fatalf("binary.Read failed: %v", err)
	}
	consumed := 4096 - r.Len()
	if consumed != kinfoSize {
		t.Fatalf("binary.Read consumed %d bytes, want %d", consumed, kinfoSize)
	}
	_ = got // unsafe.Sizeof is informational; binary.Read is authoritative
}

// TestKinfoProcFieldOffsets writes known values at the expected byte
// offsets into a raw buffer, then decodes it with binary.Read and
// verifies each exported field got the right value.
func TestKinfoProcFieldOffsets(t *testing.T) {
	buf := make([]byte, kinfoSize)
	le := binary.LittleEndian

	// StructSize at offset 0 (int32)
	le.PutUint32(buf[0:], uint32(kinfoSize))

	// Pid at offset 72 (int32)
	le.PutUint32(buf[72:], 12345)

	// Ppid at offset 76 (int32)
	le.PutUint32(buf[76:], 1)

	// Rssize at offset 264 (int64) — RSS in pages
	le.PutUint64(buf[264:], 8192)

	// Pctcpu at offset 308 (uint32)
	le.PutUint32(buf[308:], 1024)

	// Runtime at offset 328 (uint64) — microseconds
	le.PutUint64(buf[328:], 5_000_000)

	// Stat at offset 388 (int8) — SRUN = 2
	buf[388] = 2

	// Comm at offset 447 ([20]byte) — null-terminated "sshd"
	copy(buf[447:], "sshd\x00")

	// NumThreads at offset 596 (int32)
	le.PutUint32(buf[596:], 4)

	var kp kinfoProc
	if err := binary.Read(bytes.NewReader(buf), binary.LittleEndian, &kp); err != nil {
		t.Fatalf("binary.Read failed: %v", err)
	}

	if kp.StructSize != int32(kinfoSize) {
		t.Errorf("StructSize = %d, want %d", kp.StructSize, kinfoSize)
	}
	if kp.Pid != 12345 {
		t.Errorf("Pid = %d, want 12345", kp.Pid)
	}
	if kp.Rssize != 8192 {
		t.Errorf("Rssize = %d, want 8192", kp.Rssize)
	}
	if kp.Pctcpu != 1024 {
		t.Errorf("Pctcpu = %d, want 1024", kp.Pctcpu)
	}
	if kp.Runtime != 5_000_000 {
		t.Errorf("Runtime = %d, want 5000000", kp.Runtime)
	}
	if kp.Stat != 2 {
		t.Errorf("Stat = %d, want 2 (SRUN)", kp.Stat)
	}
	comm := byteSliceToString(kp.Comm[:])
	if comm != "sshd" {
		t.Errorf("Comm = %q, want %q", comm, "sshd")
	}
	if kp.NumThreads != 4 {
		t.Errorf("NumThreads = %d, want 4", kp.NumThreads)
	}
}

// TestKinfoProcMultipleEntries simulates reading several kinfo_proc entries
// from a contiguous buffer, as returned by kern.proc.all.
func TestKinfoProcMultipleEntries(t *testing.T) {
	const nProcs = 5
	buf := make([]byte, kinfoSize*nProcs)
	le := binary.LittleEndian

	for i := range nProcs {
		base := i * kinfoSize
		le.PutUint32(buf[base+0:], uint32(kinfoSize))    // StructSize
		le.PutUint32(buf[base+72:], uint32(100+i))       // Pid
		le.PutUint32(buf[base+76:], 1)                   // Ppid
		le.PutUint64(buf[base+264:], uint64(1000*(i+1))) // Rssize
		buf[base+388] = 3                                // Stat = SSLEEP
		name := []byte("proc\x00")
		copy(buf[base+447:], name)
		le.PutUint32(buf[base+596:], uint32(i+1)) // NumThreads
	}

	reader := bytes.NewReader(buf)
	var pids []int32
	for reader.Len() >= kinfoSize {
		var kp kinfoProc
		if err := binary.Read(reader, binary.LittleEndian, &kp); err != nil {
			t.Fatalf("binary.Read entry %d failed: %v", len(pids), err)
		}
		if kp.StructSize != int32(kinfoSize) {
			t.Errorf("entry %d: StructSize = %d, want %d", len(pids), kp.StructSize, kinfoSize)
		}
		pids = append(pids, kp.Pid)
	}
	if len(pids) != nProcs {
		t.Fatalf("decoded %d entries, want %d", len(pids), nProcs)
	}
	for i, pid := range pids {
		if pid != int32(100+i) {
			t.Errorf("entry %d: Pid = %d, want %d", i, pid, 100+i)
		}
	}
}

// TestKinfoProcOversizedEntry verifies that entries larger than our struct
// (future FreeBSD versions) can be handled by skipping extra bytes.
func TestKinfoProcOversizedEntry(t *testing.T) {
	const realSize = 1088 // hypothetical full struct size
	buf := make([]byte, realSize)
	le := binary.LittleEndian

	le.PutUint32(buf[0:], uint32(realSize)) // StructSize reports actual size
	le.PutUint32(buf[72:], 42)              // Pid
	le.PutUint32(buf[596:], 7)              // NumThreads

	reader := bytes.NewReader(buf)
	var kp kinfoProc
	if err := binary.Read(reader, binary.LittleEndian, &kp); err != nil {
		t.Fatalf("binary.Read failed: %v", err)
	}

	// We consumed kinfoSize bytes, but StructSize says realSize.
	// Caller must skip the difference.
	remaining := reader.Len()
	expectedRemaining := realSize - kinfoSize
	if remaining != expectedRemaining {
		t.Errorf("remaining = %d, want %d (to skip)", remaining, expectedRemaining)
	}
	if kp.Pid != 42 {
		t.Errorf("Pid = %d, want 42", kp.Pid)
	}
	if kp.NumThreads != 7 {
		t.Errorf("NumThreads = %d, want 7", kp.NumThreads)
	}
}

// TestKinfoProcCommMaxLength verifies that a full-length command name
// (19 chars + null) is correctly extracted.
func TestKinfoProcCommMaxLength(t *testing.T) {
	buf := make([]byte, kinfoSize)
	le := binary.LittleEndian
	le.PutUint32(buf[0:], uint32(kinfoSize))

	// COMMLEN = 19, so ki_comm is [20]byte.
	// Fill with 19 chars + null terminator.
	maxName := strings.Repeat("x", 19)
	copy(buf[447:], maxName+"\x00")

	var kp kinfoProc
	if err := binary.Read(bytes.NewReader(buf), binary.LittleEndian, &kp); err != nil {
		t.Fatalf("binary.Read failed: %v", err)
	}
	comm := byteSliceToString(kp.Comm[:])
	if comm != maxName {
		t.Errorf("Comm = %q (len %d), want %q (len 19)", comm, len(comm), maxName)
	}
}

// TestKinfoProcNegativeValues verifies signed fields decode correctly
// with negative values (e.g., Rssize is segsz_t = int64).
func TestKinfoProcNegativeValues(t *testing.T) {
	buf := make([]byte, kinfoSize)
	le := binary.LittleEndian
	le.PutUint32(buf[0:], uint32(kinfoSize))

	// Stat = -1 (invalid state, but tests signed decode)
	buf[388] = 0xFF // -1 as int8

	var kp kinfoProc
	if err := binary.Read(bytes.NewReader(buf), binary.LittleEndian, &kp); err != nil {
		t.Fatalf("binary.Read failed: %v", err)
	}
	if kp.Stat != -1 {
		t.Errorf("Stat = %d, want -1", kp.Stat)
	}
}

// byteSliceToString is the same helper used in production code.
// Included here in case the test file needs to compile standalone.
func byteSliceToString(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}

// BenchmarkKinfoProcDecode measures the cost of decoding 200 kinfo_proc
// entries from a contiguous buffer (realistic for a busy system).
func BenchmarkKinfoProcDecode(b *testing.B) {
	const nProcs = 200
	buf := make([]byte, kinfoSize*nProcs)
	le := binary.LittleEndian
	for i := range nProcs {
		base := i * kinfoSize
		le.PutUint32(buf[base+0:], uint32(kinfoSize))
		le.PutUint32(buf[base+72:], uint32(i+1))
		buf[base+388] = 3
		copy(buf[base+447:], "bench\x00")
		le.PutUint32(buf[base+596:], 1)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		reader := bytes.NewReader(buf)
		for reader.Len() >= kinfoSize {
			var kp kinfoProc
			if err := binary.Read(reader, binary.LittleEndian, &kp); err != nil {
				b.Fatal(err)
			}
		}
	}
}

// TestCollectProcessRaw_Integration calls the real kern.proc.all sysctl
// and prints decoded process data for manual inspection.
func TestCollectProcessRaw_Integration(t *testing.T) {
	procs, totalMem, err := collectProcessRaw()
	if err != nil {
		t.Fatalf("collectProcessRaw: %v", err)
	}

	if len(procs) == 0 {
		t.Fatal("expected at least 1 process")
	}

	t.Logf("Total memory: %d bytes (%.1f GiB)", totalMem, float64(totalMem)/(1024*1024*1024))
	t.Logf("Process count: %d", len(procs))
	t.Logf("")
	t.Logf("%-8s %-20s %-6s %12s %16s %8s", "PID", "NAME", "STATE", "RSS (KiB)", "RUNTIME (µs)", "THREADS")
	t.Logf("%-8s %-20s %-6s %12s %16s %8s", "---", "----", "-----", "---------", "------------", "-------")

	for _, p := range procs {
		t.Logf("%-8d %-20s %-6s %12d %16d %8d",
			p.PID,
			truncate(p.Name, 20),
			p.State,
			p.RSSBytes/1024,
			p.TotalTicks,
			p.NumThreads,
		)
	}
}

// TestCollectProcessRaw_SanityChecks runs basic assertions against
// live process data to catch struct misalignment.
func TestCollectProcessRaw_SanityChecks(t *testing.T) {
	procs, totalMem, err := collectProcessRaw()
	if err != nil {
		t.Fatalf("collectProcessRaw: %v", err)
	}

	if totalMem <= 0 {
		t.Errorf("totalMem = %d, want > 0", totalMem)
	}

	if len(procs) < 5 {
		t.Errorf("expected at least 5 processes, got %d", len(procs))
	}

	foundInit := false
	foundSelf := false
	validStates := map[string]bool{"R": true, "S": true, "T": true, "Z": true, "?": true}

	for _, p := range procs {
		// PID 1 should always exist
		if p.PID == 1 {
			foundInit = true
			if p.Name == "" {
				t.Error("PID 1 has empty name")
			}
		}

		// Our own test process should be running
		if p.Name == "go" || p.Name == "process.test" || p.Name == "collector.test" {
			foundSelf = true
		}

		// PIDs must be positive
		if p.PID < 0 {
			t.Errorf("invalid PID: %d (name=%q)", p.PID, p.Name)
		}

		// State must be one of our mapped values
		if !validStates[p.State] {
			t.Errorf("PID %d (%s): unexpected state %q", p.PID, p.Name, p.State)
		}

		// Thread count must be at least 1
		if p.NumThreads < 1 {
			t.Errorf("PID %d (%s): NumThreads = %d, want >= 1", p.PID, p.Name, p.NumThreads)
		}

		// RSS should not exceed total memory
		if p.RSSBytes > uint64(totalMem) {
			t.Errorf("PID %d (%s): RSS %d > totalMem %d", p.PID, p.Name, p.RSSBytes, totalMem)
		}

		// Command name should be non-empty and reasonable length
		if len(p.Name) == 0 {
			t.Errorf("PID %d: empty process name", p.PID)
		}
		if len(p.Name) > 19 {
			t.Errorf("PID %d: name %q exceeds COMMLEN (19)", p.PID, p.Name)
		}
	}

	if !foundInit {
		t.Error("PID 1 not found in process list")
	}
	if !foundSelf {
		t.Log("warning: could not identify own test process (not fatal)")
	}
}

// TestCollectProcesses_Integration runs the full shared pipeline
// including delta math and normalizeProcState.
func TestCollectProcesses_Integration(t *testing.T) {
	// First call: establishes baseline (no CPU deltas yet)
	metrics1, err := CollectProcesses(context.Background())
	if err != nil {
		t.Fatalf("first CollectProcesses: %v", err)
	}
	if len(metrics1) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(metrics1))
	}

	t.Logf("First collection: %d processes (all CPU%% will be 0)", countProcesses(t, metrics1))

	// Second call: should produce real CPU deltas
	metrics2, err := CollectProcesses(context.Background())
	if err != nil {
		t.Fatalf("second CollectProcesses: %v", err)
	}

	count := countProcesses(t, metrics2)
	t.Logf("Second collection: %d processes", count)

	// Print a sample of the final metrics
	if pl, ok := metrics2[0].(protocol.ProcessListMetric); ok {
		t.Logf("")
		t.Logf("%-8s %-20s %-10s %8s %8s %8s", "PID", "NAME", "STATUS", "CPU%", "MEM%", "THREADS")
		t.Logf("%-8s %-20s %-10s %8s %8s %8s", "---", "----", "------", "----", "----", "-------")

		limit := 30
		for i, p := range pl.Processes {
			if i >= limit {
				t.Logf("... (%d more)", len(pl.Processes)-limit)
				break
			}
			t.Logf("%-8d %-20s %-10s %8.2f %8.2f %8d",
				p.Pid,
				truncate(p.Name, 20),
				p.Status,
				p.CPUPercent,
				p.MemPercent,
				p.ThreadsTotal,
			)
		}
	}
}

func countProcesses(t *testing.T, metrics []protocol.Metric) int {
	t.Helper()
	if len(metrics) == 0 {
		return 0
	}
	if pl, ok := metrics[0].(protocol.ProcessListMetric); ok {
		return len(pl.Processes)
	}
	t.Error("first metric is not ProcessListMetric")
	return 0
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
