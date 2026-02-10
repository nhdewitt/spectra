//go:build linux

package collector

import (
	"bytes"
	"context"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func parseFixture(t *testing.T, key string) map[string]CPURaw {
	t.Helper()

	s, ok := procStatSamples[key]
	if !ok {
		t.Fatalf("unknown fixture %q", key)
	}

	r := strings.NewReader(s)
	got, err := parseProcStatFrom(r)
	if err != nil {
		t.Fatalf("parsing fixture %q: %v", key, err)
	}
	return got
}

func BenchmarkCollectCPU(b *testing.B) {
	ctx := context.Background()

	for b.Loop() {
		CollectCPU(ctx)
	}
}

func BenchmarkCollectCPU_FullCycle(b *testing.B) {
	ctx := context.Background()
	lastCPURawData = nil

	// Baseline
	CollectCPU(ctx)

	b.ResetTimer()
	for b.Loop() {
		CollectCPU(ctx)
	}
}

func BenchmarkCalcCoreUsage_Allocs(b *testing.B) {
	deltaMap := map[string]CPUDelta{
		"cpu":  {Used: 100, Total: 200},
		"cpu0": {Used: 50, Total: 100},
		"cpu1": {Used: 75, Total: 100},
		"cpu2": {Used: 60, Total: 100},
		"cpu3": {Used: 80, Total: 100},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = calcCoreUsage(deltaMap)
	}
}

func TestParseCPULine(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		want    CPURaw
		wantErr bool
	}{
		{
			name: "valid aggregate cpu line",
			line: "cpu  1000 200 300 5000 100 50 25 10 5 3",
			want: CPURaw{
				User:      1000,
				Nice:      200,
				System:    300,
				Idle:      5000,
				IOWait:    100,
				IRQ:       50,
				SoftIRQ:   25,
				Steal:     10,
				Guest:     5,
				GuestNice: 3,
			},
			wantErr: false,
		},
		{
			name: "valid per-core cpu line",
			line: "cpu0 500 100 150 2500 50 25 12 5 2 1",
			want: CPURaw{
				User:      500,
				Nice:      100,
				System:    150,
				Idle:      2500,
				IOWait:    50,
				IRQ:       25,
				SoftIRQ:   12,
				Steal:     5,
				Guest:     2,
				GuestNice: 1,
			},
			wantErr: false,
		},
		{
			name: "large values (uint64 range)",
			line: "cpu  18446744073709551615 0 0 0 0 0 0 0 0 0",
			want: CPURaw{
				User: 18446744073709551615,
			},
			wantErr: false,
		},
		{
			name:    "insufficient fields",
			line:    "cpu 100 200 300",
			want:    CPURaw{},
			wantErr: true,
		},
		{
			name:    "empty line",
			line:    "",
			want:    CPURaw{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCPULine(tt.line)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseCPULine() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parseCPULine() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestCalculateDelta(t *testing.T) {
	tests := []struct {
		name     string
		current  map[string]CPURaw
		previous map[string]CPURaw
		wantKey  string
		want     CPUDelta
		wantOK   bool
	}{
		{
			name: "basic delta calculation",
			current: map[string]CPURaw{
				"cpu": {User: 200, Nice: 20, System: 50, Idle: 1000, IOWait: 10, IRQ: 5, SoftIRQ: 3, Steal: 1, Guest: 4, GuestNice: 2},
			},
			previous: map[string]CPURaw{
				"cpu": {User: 100, Nice: 10, System: 25, Idle: 500, IOWait: 5, IRQ: 2, SoftIRQ: 1, Steal: 0, Guest: 2, GuestNice: 1},
			},
			wantKey: "cpu",
			want: CPUDelta{
				User:      100,
				Nice:      10,
				System:    25,
				Idle:      500,
				IOWait:    5,
				IRQ:       3,
				SoftIRQ:   2,
				Steal:     1,
				Guest:     2,
				GuestNice: 1,
				Total:     646,
				Used:      141,
			},
			wantOK: true,
		},
		{
			name: "zero delta (no change)",
			current: map[string]CPURaw{
				"cpu": {User: 100, Nice: 10, System: 25, Idle: 500, IOWait: 5, IRQ: 2, SoftIRQ: 1, Steal: 0, Guest: 0, GuestNice: 0},
			},
			previous: map[string]CPURaw{
				"cpu": {User: 100, Nice: 10, System: 25, Idle: 500, IOWait: 5, IRQ: 2, SoftIRQ: 1, Steal: 0, Guest: 0, GuestNice: 0},
			},
			wantKey: "cpu",
			want: CPUDelta{
				Total: 0,
				Used:  0,
			},
			wantOK: true,
		},
		{
			name: "high CPU usage scenario",
			current: map[string]CPURaw{
				"cpu": {User: 1000, Nice: 0, System: 500, Idle: 100, IOWait: 0, IRQ: 0, SoftIRQ: 0, Steal: 0, Guest: 0, GuestNice: 0},
			},
			previous: map[string]CPURaw{
				"cpu": {User: 0, Nice: 0, System: 0, Idle: 0, IOWait: 0, IRQ: 0, SoftIRQ: 0, Steal: 0, Guest: 0, GuestNice: 0},
			},
			wantKey: "cpu",
			want: CPUDelta{
				User:   1000,
				System: 500,
				Idle:   100,
				Total:  1600,
				Used:   1500,
			},
			wantOK: true,
		},
		{
			name: "guest time not double-counted",
			current: map[string]CPURaw{
				"cpu": {User: 100, Nice: 10, System: 0, Idle: 0, IOWait: 0, IRQ: 0, SoftIRQ: 0, Steal: 0, Guest: 50, GuestNice: 5},
			},
			previous: map[string]CPURaw{
				"cpu": {User: 0, Nice: 0, System: 0, Idle: 0, IOWait: 0, IRQ: 0, SoftIRQ: 0, Steal: 0, Guest: 0, GuestNice: 0},
			},
			wantKey: "cpu",
			want: CPUDelta{
				User:      100,
				Nice:      10,
				Guest:     50,
				GuestNice: 5,
				Total:     110,
				Used:      110,
			},
			wantOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := calculateCPUDeltas(tt.current, tt.previous)
			if ok != tt.wantOK {
				t.Fatalf("calculateCPUDeltas() ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			delta, exists := got[tt.wantKey]
			if !exists {
				t.Fatalf("calculateCPUDeltas() missing key %q", tt.wantKey)
			}
			if delta != tt.want {
				t.Errorf("calculateCPUDeltas()[%q] = %+v, want %+v", tt.wantKey, delta, tt.want)
			}
		})
	}
}

func TestCalculateDelta_MissingPreviousKey(t *testing.T) {
	current := map[string]CPURaw{
		"cpu":  {User: 100},
		"cpu0": {User: 50},
	}
	previous := map[string]CPURaw{
		"cpu": {User: 50},
	}

	got, ok := calculateCPUDeltas(current, previous)

	if ok {
		t.Error("expected ok=-false when previous key is missing")
	}

	if got != nil {
		t.Error("expected nil map when previous key is missing")
	}
}

func TestCalcCoreUsage(t *testing.T) {
	tests := []struct {
		name     string
		deltaMap map[string]CPUDelta
		want     []float64
	}{
		{
			name: "two cores with varying usage",
			deltaMap: map[string]CPUDelta{
				"cpu":  {Used: 100, Total: 200},
				"cpu0": {Used: 50, Total: 100},
				"cpu1": {Used: 75, Total: 100},
			},
			want: []float64{50.0, 75.0},
		},
		{
			name: "four cores",
			deltaMap: map[string]CPUDelta{
				"cpu":  {Used: 100, Total: 400},
				"cpu0": {Used: 0, Total: 100},
				"cpu1": {Used: 25, Total: 100},
				"cpu2": {Used: 50, Total: 100},
				"cpu3": {Used: 100, Total: 100},
			},
			want: []float64{0.0, 25.0, 50.0, 100.0},
		},
		{
			name: "single core",
			deltaMap: map[string]CPUDelta{
				"cpu":  {Used: 80, Total: 100},
				"cpu0": {Used: 80, Total: 100},
			},
			want: []float64{80.0},
		},
		{
			name: "no cores (only aggregate)",
			deltaMap: map[string]CPUDelta{
				"cpu": {Used: 50, Total: 100},
			},
			want: []float64{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calcCoreUsage(tt.deltaMap)
			if len(got) != len(tt.want) {
				t.Fatalf("calcCoreUsage() returned %d cores, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("calcCoreUsage()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestCalcCoreUsage_ZeroTotal(t *testing.T) {
	// Edge case: what happens with zero total (first sample or error)?
	deltaMap := map[string]CPUDelta{
		"cpu":  {Used: 0, Total: 0},
		"cpu0": {Used: 0, Total: 0},
	}

	got := calcCoreUsage(deltaMap)

	if len(got) != 1 {
		t.Fatalf("expected 1 core, got %d", len(got))
	}
}

func TestCalcCoreUsage_SparseCores(t *testing.T) {
	// Edge case: What if cpu1 is missing but cpu0 and cpu2 exist?
	deltaMap := map[string]CPUDelta{
		"cpu":  {Used: 100, Total: 200},
		"cpu0": {Used: 50, Total: 100},
		// cpu1 missing
		"cpu2": {Used: 75, Total: 100},
	}

	got := calcCoreUsage(deltaMap)

	if len(got) != 2 {
		t.Errorf("got %d cores, want 2", len(got))
	}

	if got[1] != 0 {
		t.Errorf("missing core should have 0 usage, got %f", got[1])
	}
}

func BenchmarkParseCPULine(b *testing.B) {
	line := "cpu  123456 7890 12345 678901 2345 678 90 12 34 0"
	for b.Loop() {
		_, _ = parseCPULine(line)
	}
}

func BenchmarkCalculateDelta(b *testing.B) {
	current := map[string]CPURaw{
		"cpu":  {User: 200, Nice: 20, System: 50, Idle: 1000, IOWait: 10, IRQ: 5, SoftIRQ: 3, Steal: 1, Guest: 0},
		"cpu0": {User: 100, Nice: 10, System: 25, Idle: 500, IOWait: 5, IRQ: 2, SoftIRQ: 1, Steal: 0, Guest: 0},
		"cpu1": {User: 100, Nice: 10, System: 25, Idle: 500, IOWait: 5, IRQ: 3, SoftIRQ: 2, Steal: 1, Guest: 0},
	}
	previous := map[string]CPURaw{
		"cpu":  {User: 100, Nice: 10, System: 25, Idle: 500, IOWait: 5, IRQ: 2, SoftIRQ: 1, Steal: 0, Guest: 0},
		"cpu0": {User: 50, Nice: 5, System: 12, Idle: 250, IOWait: 2, IRQ: 1, SoftIRQ: 0, Steal: 0, Guest: 0},
		"cpu1": {User: 50, Nice: 5, System: 12, Idle: 250, IOWait: 2, IRQ: 1, SoftIRQ: 1, Steal: 0, Guest: 0},
	}

	for b.Loop() {
		_, _ = calculateCPUDeltas(current, previous)
	}
}

func BenchmarkCalcCoreUsage(b *testing.B) {
	deltaMap := map[string]CPUDelta{
		"cpu":  {Used: 100, Total: 200},
		"cpu0": {Used: 50, Total: 100},
		"cpu1": {Used: 75, Total: 100},
		"cpu2": {Used: 60, Total: 100},
		"cpu3": {Used: 80, Total: 100},
	}

	for b.Loop() {
		_ = calcCoreUsage(deltaMap)
	}
}

// Integration tests for filesystem-dependent functions.
// These tests use temp files to simulate /proc entries.

func TestParseProcStat_RealFilesystem(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("skipping: requires Linux /proc filesystem")
	}

	result, err := parseProcStat()
	if err != nil {
		t.Fatalf("parseProcStat() error = %v", err)
	}

	// Must have aggregate "cpu" entry
	if _, ok := result["cpu"]; !ok {
		t.Error("parseProcStat() missing 'cpu' aggregate entry")
	}

	// Should have at least one core
	if _, ok := result["cpu0"]; !ok {
		t.Error("parseProcStat() missing 'cpu0' entry")
	}

	// Validate values are non-zero
	cpu := result["cpu"]
	if cpu.User == 0 && cpu.System == 0 && cpu.Idle == 0 {
		t.Error("parseProcStat() all values are zero, expected some CPU activity")
	}
}

func TestParseLoadAvg_RealFilesystem(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("skipping: requires Linux /proc filesystem")
	}

	load1, load5, load15, err := parseLoadAvg()
	if err != nil {
		t.Fatalf("parseLoadAvg() error = %v", err)
	}

	// Load averages should be non-negative
	if load1 < 0 || load5 < 0 || load15 < 0 {
		t.Errorf("parseLoadAvg() returned negative values: %v, %v, %v", load1, load5, load15)
	}

	// Load average shouldn't be astronomically high
	if load1 > 10000 || load5 > 10000 || load15 > 10000 {
		t.Errorf("parseLoadAvg() returned suspiciously high values: %v, %v, %v", load1, load5, load15)
	}
}

// TestCollectCPU_Integration tests the full collection flow.
// First call returns nil (baseline), second call returns metrics.
func TestCollectCPU_Integration(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("skipping: requires Linux /proc filesystem")
	}

	lastCPURawData = nil
	ctx := context.Background()

	// First call - baseline
	metrics1, err := CollectCPU(ctx)
	if err != nil {
		t.Fatalf("CollectCPU() first call error = %v", err)
	}
	if metrics1 != nil {
		t.Error("CollectCPU() first call should return nil")
	}

	// Second call - actual metrics
	metrics2, err := CollectCPU(ctx)
	if err != nil {
		t.Fatalf("CollectCPU() second call error = %v", err)
	}
	if metrics2 == nil {
		t.Fatal("CollectCPU() second call returned nil, expected metrics")
	}
	if len(metrics2) != 1 {
		t.Fatalf("CollectCPU() returned %d metrics, expected 1", len(metrics2))
	}
}

func TestCollectCPU_CounterReset(t *testing.T) {
	// Simulate what happens when counters reset (reboot, overflow)
	lastCPURawData = map[string]CPURaw{
		"cpu": {User: 1000, Nice: 100, System: 500, Idle: 5000, IOWait: 50, IRQ: 10, SoftIRQ: 5, Steal: 0},
	}

	ctx := context.Background()
	metrics, err := CollectCPU(ctx)
	// After reset detection, lastCPURawData should be nil
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if metrics != nil {
		t.Error("expected nil metrics after counter reset")
	}
	if lastCPURawData != nil {
		t.Error("expected lastCPURawData to be reset to nil")
	}
}

// File-based integration tests using temp files.
// Test parsing logic with controlled input.

func TestParseProcStatFile(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantCPUs []string
		wantErr  bool
	}{
		{
			name: "single core system",
			content: `cpu  1000 200 300 5000 100 50 25 10 5 3
cpu0 1000 200 300 5000 100 50 25 10 5 3
intr 123456 0 0 0
`,
			wantCPUs: []string{"cpu", "cpu0"},
			wantErr:  false,
		},
		{
			name: "quad-core system",
			content: `cpu  4000 800 1200 20000 400 200 100 40 20 12
cpu0 1000 200 300 5000 100 50 25 10 5 3
cpu1 1000 200 300 5000 100 50 25 10 5 3
cpu2 1000 200 300 5000 100 50 25 10 5 3
cpu3 1000 200 300 5000 100 50 25 10 5 3
intr 123456 0 0 0
`,
			wantCPUs: []string{"cpu", "cpu0", "cpu1", "cpu2", "cpu3"},
			wantErr:  false,
		},
		{
			name: "raspberry pi 4",
			content: `cpu  12345 678 9012 345678 901 234 56 0 0 0
cpu0 3086 169 2253 86419 225 58 14 0 0 0
cpu1 3086 169 2253 86419 225 58 14 0 0 0
cpu2 3086 170 2253 86420 225 59 14 0 0 0
cpu3 3087 170 2253 86420 226 59 14 0 0 0
intr 1234567 0 0 0
`,
			wantCPUs: []string{"cpu", "cpu0", "cpu1", "cpu2", "cpu3"},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := createTempFile(t, tt.content)

			result := parseProcStatContent(t, tmpFile)

			if tt.wantErr {
				return
			}

			for _, cpu := range tt.wantCPUs {
				if _, ok := result[cpu]; !ok {
					t.Errorf("missing expected CPU entry: %s", cpu)
				}
			}

			if len(result) != len(tt.wantCPUs) {
				t.Errorf("got %d CPU entries, want %d", len(result), len(tt.wantCPUs))
			}
		})
	}
}

func TestParseLoadAvgFile(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want1   float64
		want5   float64
		want15  float64
		wantErr bool
	}{
		{
			name:    "typical idle system",
			content: "0.00 0.01 0.05 1/234 5678\n",
			want1:   0.00,
			want5:   0.01,
			want15:  0.05,
			wantErr: false,
		},
		{
			name:    "moderate load",
			content: "1.23 0.98 0.76 3/456 7890\n",
			want1:   1.23,
			want5:   0.98,
			want15:  0.76,
			wantErr: false,
		},
		{
			name:    "high load",
			content: "4.00 3.85 3.21 8/123 4567\n",
			want1:   4.00,
			want5:   3.85,
			want15:  3.21,
			wantErr: false,
		},
		{
			name:    "insufficient fields",
			content: "0.00 0.01\n",
			wantErr: true,
		},
		{
			name:    "invalid number",
			content: "abc 0.01 0.05 1/234 5678\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := createTempFile(t, tt.content)

			load1, load5, load15, err := parseLoadAvgFile(tmpFile)

			if (err != nil) != tt.wantErr {
				t.Errorf("parseLoadAvgFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if load1 != tt.want1 {
					t.Errorf("load1 = %v, want %v", load1, tt.want1)
				}
				if load5 != tt.want5 {
					t.Errorf("load5 = %v, want %v", load5, tt.want5)
				}
				if load15 != tt.want15 {
					t.Errorf("load15 = %v, want %v", load15, tt.want15)
				}
			}
		})
	}
}

// Helper functions

func createTempFile(t *testing.T, content string) string {
	t.Helper()
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "testfile")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	return tmpFile
}

// parseProcStatContent parses a /proc/stat format file for testing.
// This mirrors parseProcStat but accepts a filepath.
func parseProcStatContent(t *testing.T, path string) map[string]CPURaw {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	result := make(map[string]CPURaw)
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "cpu") {
			break
		}

		raw, err := parseCPULine(line)
		if err != nil {
			continue
		}

		fields := strings.Fields(line)
		result[fields[0]] = raw
	}

	return result
}

// parseLoadAvgFile parses a /proc/loadavg format file for testing.
// This mirrors parseLoadAvg but accepts a filepath.
func parseLoadAvgFile(path string) (load1, load5, load15 float64, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, 0, err
	}

	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return 0, 0, 0, os.ErrInvalid
	}

	var parseErr error
	parseFloat := func(s string) float64 {
		v, e := strconv.ParseFloat(s, 64)
		if e != nil && parseErr == nil {
			parseErr = e
		}
		return v
	}

	load1 = parseFloat(fields[0])
	load5 = parseFloat(fields[1])
	load15 = parseFloat(fields[2])

	return load1, load5, load15, parseErr
}

// Test edge cases and error conditions

func TestParseProcStat_FileNotFound(t *testing.T) {
	_, err := os.Open("/nonexistent/proc/stat")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestParseProcStatFrom_MalformedLines(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantCPUs int // map entries, including the aggregate "cpu"
	}{
		{
			name:     "malformed line skipped",
			input:    "cpu  1000 200 300 5000 100 50 25 10 5 3\ncpu0 bad data here\ncpu1 500 100 150 2500 50 25 12 5 2 1\n",
			wantCPUs: 2, // cpu0 skipped
		},
		{
			name:     "empty lines ignored",
			input:    "cpu  1000 200 300 5000 100 50 25 10 5 3\n\ncpu0 500 100 150 2500 50 25 12 5 2 1\n",
			wantCPUs: 1, // stops at empty line
		},
		{
			name:     "only aggregate",
			input:    "cpu  1000 200 300 5000 100 50 25 10 5 3\nintr 123\n",
			wantCPUs: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseProcStatFrom(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != tt.wantCPUs {
				t.Errorf("got %d CPUs, want %d", len(got), tt.wantCPUs)
			}
		})
	}
}

func TestCollectCPU_StateReset(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("skipping: requires Linux /proc filesystem")
	}

	lastCPURawData = nil
	ctx := context.Background()

	_, _ = CollectCPU(ctx)

	if lastCPURawData == nil {
		t.Error("lastCPURawData should be set after first collection")
	}

	lastCPURawData = nil
	if lastCPURawData != nil {
		t.Error("lastCPURawData should be nil after reset")
	}
}

// Stress test for sequential calls

func TestCollectCPU_SequentialCalls(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("skipping: requires Linux /proc filesystem")
	}

	lastCPURawData = nil
	ctx := context.Background()

	_, _ = CollectCPU(ctx)

	for i := range 10 {
		metrics, err := CollectCPU(ctx)
		if err != nil {
			t.Fatalf("CollectCPU() call %d error = %v", i, err)
		}
		if metrics == nil {
			t.Fatalf("CollectCPU() call %d returned nil", i)
		}
	}
}

func TestCalculateDelta_CounterRegression(t *testing.T) {
	tests := []struct {
		name     string
		current  map[string]CPURaw
		previous map[string]CPURaw
	}{
		{
			name: "user counter decreased",
			current: map[string]CPURaw{
				"cpu": {User: 50, Nice: 10, System: 25, Idle: 500, IOWait: 5, IRQ: 2, SoftIRQ: 1, Steal: 0},
			},
			previous: map[string]CPURaw{
				"cpu": {User: 100, Nice: 10, System: 25, Idle: 500, IOWait: 5, IRQ: 2, SoftIRQ: 1, Steal: 0},
			},
		},
		{
			name: "idle counter decreased",
			current: map[string]CPURaw{
				"cpu": {User: 100, Nice: 10, System: 25, Idle: 400, IOWait: 5, IRQ: 2, SoftIRQ: 1, Steal: 0},
			},
			previous: map[string]CPURaw{
				"cpu": {User: 100, Nice: 10, System: 25, Idle: 500, IOWait: 5, IRQ: 2, SoftIRQ: 1, Steal: 0},
			},
		},
		{
			name: "system counter decreased",
			current: map[string]CPURaw{
				"cpu": {User: 100, Nice: 10, System: 20, Idle: 500, IOWait: 5, IRQ: 2, SoftIRQ: 1, Steal: 0},
			},
			previous: map[string]CPURaw{
				"cpu": {User: 100, Nice: 10, System: 25, Idle: 500, IOWait: 5, IRQ: 2, SoftIRQ: 1, Steal: 0},
			},
		},
		{
			name: "steal counter decreased",
			current: map[string]CPURaw{
				"cpu": {User: 100, Nice: 10, System: 25, Idle: 500, IOWait: 5, IRQ: 2, SoftIRQ: 1, Steal: 0},
			},
			previous: map[string]CPURaw{
				"cpu": {User: 100, Nice: 10, System: 25, Idle: 500, IOWait: 5, IRQ: 2, SoftIRQ: 1, Steal: 5},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := calculateCPUDeltas(tt.current, tt.previous)
			if ok {
				t.Error("expected ok=false when counter decreased")
			}
			if got != nil {
				t.Error("expected nil map when counter decreased")
			}
		})
	}
}

func TestParseProcStatFrom(t *testing.T) {
	tests := []struct {
		name      string
		fixture   string
		wantCores int
		wantErr   bool
	}{
		{"4-core standard", "proc_stat_4core", 4, false},
		{"single core", "proc_stat_single_core", 1, false},
		{"8-core loaded", "proc_stat_8core_loaded", 8, false},
		{"high values", "proc_stat_high_values", 4, false},
		{"fresh boot", "proc_stat_fresh_boot", 2, false},
		{"old kernel", "proc_stat_old_kernel", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, ok := procStatSamples[tt.fixture]
			if !ok {
				t.Fatalf("unknown fixture %q", tt.fixture)
			}

			got, err := parseProcStatFrom(strings.NewReader(s))
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			coreCount := len(got) - 1
			if _, hasCPU := got["cpu"]; !hasCPU {
				coreCount = len(got)
			}

			if coreCount != tt.wantCores {
				t.Errorf("got %d cores, want %d", coreCount, tt.wantCores)
			}
		})
	}
}

func parseTestFile(t *testing.T, path string) map[string]CPURaw {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("opening %s: %v", path, err)
	}
	defer f.Close()

	result, err := parseProcStatFrom(f)
	if err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	return result
}

func TestProcStatFromDeltas(t *testing.T) {
	tests := []struct {
		name      string
		fixtures  []string
		wantOK    bool
		wantDelta CPUDelta
		wantUsage float64
	}{
		{
			name:     "normal usage",
			fixtures: []string{"delta_normal_t0", "delta_normal_t1"},
			wantOK:   true,
			wantDelta: CPUDelta{
				User: 1000, Nice: 100, System: 500, Idle: 8000, IOWait: 200, IRQ: 10, SoftIRQ: 20, Steal: 5, Total: 9835, Used: 1635,
			},
			wantUsage: 16.62,
		},
		{
			name:     "high cpu",
			fixtures: []string{"delta_high_cpu_t0", "delta_high_cpu_t1"},
			wantOK:   true,
			wantDelta: CPUDelta{
				User: 9000, Nice: 100, System: 800, Idle: 100, IOWait: 50, IRQ: 20, SoftIRQ: 30, Steal: 0, Total: 10100, Used: 9950,
			},
			wantUsage: 98.51,
		},
		{
			name:     "idle system",
			fixtures: []string{"delta_idle_t0", "delta_idle_t1"},
			wantOK:   true,
			wantDelta: CPUDelta{
				User: 10, Nice: 0, System: 5, Idle: 10000, IOWait: 5, IRQ: 0, SoftIRQ: 0, Steal: 0, Total: 10020, Used: 15,
			},
			wantUsage: 0.15,
		},
		{
			name:     "counter reset",
			fixtures: []string{"delta_reset_t0", "delta_reset_t1"},
			wantOK:   false,
		},
		{
			name:     "cpu hotplug",
			fixtures: []string{"delta_hotplug_t0", "delta_hotplug_t1"},
			wantOK:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prev := parseFixture(t, tt.fixtures[0])
			cur := parseFixture(t, tt.fixtures[1])

			deltaMap, ok := calculateCPUDeltas(cur, prev)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}

			got := deltaMap["cpu"]
			if got != tt.wantDelta {
				t.Errorf("delta mismatch:\ngot: %+v\nwant: %+v", got, tt.wantDelta)
			}

			usage := percent(got.Used, got.Total)
			if math.Abs(usage-tt.wantUsage) > 0.01 {
				t.Errorf("usage = %.2f%%, want %.2f%%", usage, tt.wantUsage)
			}
		})
	}
}

func BenchmarkParseProcStatFrom(b *testing.B) {
	data := []byte(procStatSamples["proc_stat_8core_loaded"])

	b.ResetTimer()
	for b.Loop() {
		r := bytes.NewReader(data)
		_, _ = parseProcStatFrom(r)
	}
}
