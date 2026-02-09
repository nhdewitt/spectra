//go:build freebsd

package collector

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"
)

func TestParseCPUTimes_Empty(t *testing.T) {
	_, err := parseCPUTimes(nil)
	if err == nil {
		t.Fatalf("expected error for empty data, got nil")
	}
}

func TestParseCPUTimes_BadLength(t *testing.T) {
	// Not a multiple of structSize (40)
	data := make([]byte, structSize+1)
	_, err := parseCPUTimes(data)
	if err == nil {
		t.Fatalf("expected error for bad length, got nil")
	}
}

func TestParseCPUTimes_OK_MultipleEntries(t *testing.T) {
	in := []CPUTime{
		{User: 1, Nice: 2, Sys: 3, Intr: 4, Idle: 5},
		{User: 10, Nice: 20, Sys: 30, Intr: 40, Idle: 50},
	}

	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, in); err != nil {
		t.Fatalf("binary.Write: %v", err)
	}

	out, err := parseCPUTimes(buf.Bytes())
	if err != nil {
		t.Fatalf("parseCPUTimes: %v", err)
	}
	if len(out) != len(in) {
		t.Fatalf("expected %d entries, got %d", len(in), len(out))
	}

	for i := range in {
		if out[i] != in[i] {
			t.Fatalf("entry %d mismatch: got %+v want %+v", i, out[i], in[i])
		}
	}
}

func TestParseLoadAvg_TooShort(t *testing.T) {
	// Need at least 3*4 + 4 + 8 = 24 bytes
	data := make([]byte, 23)
	_, _, _, err := parseLoadAvg(data)
	if err == nil {
		t.Fatalf("expected error for too-short data, got nil")
	}
}

func TestParseLoadAvg_FscaleZero(t *testing.T) {
	raw := loadAvgRaw{
		Load:   [3]uint32{1, 2, 3},
		Fscale: 0,
	}
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, &raw); err != nil {
		t.Fatalf("binary.Write: %v", err)
	}

	_, _, _, err := parseLoadAvg(buf.Bytes())
	if err == nil {
		t.Fatalf("expected error for fscale=0, got nil")
	}
}

func TestParseLoadAvg_OK(t *testing.T) {
	raw := loadAvgRaw{
		Load:   [3]uint32{100, 250, 500},
		Fscale: 100, // so loads become 1.0, 2.5, 5.0
	}
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, &raw); err != nil {
		t.Fatalf("binary.Write: %v", err)
	}

	l1, l5, l15, err := parseLoadAvg(buf.Bytes())
	if err != nil {
		t.Fatalf("parseLoadAvg: %v", err)
	}

	assertApprox(t, l1, 1.0, 1e-12)
	assertApprox(t, l5, 2.5, 1e-12)
	assertApprox(t, l15, 5.0, 1e-12)
}

func TestCalculateCPUDeltas_MissingKey(t *testing.T) {
	cur := map[string]CPURaw{
		"cpu": {User: 2, Nice: 0, System: 0, IRQ: 0, Idle: 2},
	}
	prev := map[string]CPURaw{
		"cpu0": {User: 1, Nice: 0, System: 0, IRQ: 0, Idle: 1},
	}

	_, ok := calculateCPUDeltas(cur, prev)
	if ok {
		t.Fatalf("expected ok=false when previous missing key")
	}
}

func TestCalculateCPUDeltas_RolloverDetected(t *testing.T) {
	cur := map[string]CPURaw{
		"cpu": {User: 1, Nice: 0, System: 0, IRQ: 0, Idle: 0},
	}
	prev := map[string]CPURaw{
		"cpu": {User: 2, Nice: 0, System: 0, IRQ: 0, Idle: 0},
	}

	_, ok := calculateCPUDeltas(cur, prev)
	if ok {
		t.Fatalf("expected ok=false when current < previous (rollover/regression)")
	}
}

func TestCalculateCPUDeltas_OK_ComputesTotals(t *testing.T) {
	cur := map[string]CPURaw{
		"cpu":  {User: 15, Nice: 3, System: 7, IRQ: 2, Idle: 100},
		"cpu0": {User: 9, Nice: 1, System: 2, IRQ: 1, Idle: 40},
	}
	prev := map[string]CPURaw{
		"cpu":  {User: 10, Nice: 1, System: 4, IRQ: 1, Idle: 90},
		"cpu0": {User: 7, Nice: 1, System: 1, IRQ: 1, Idle: 35},
	}

	dm, ok := calculateCPUDeltas(cur, prev)
	if !ok {
		t.Fatalf("expected ok=true")
	}

	d := dm["cpu"]
	if d.User != 5 || d.Nice != 2 || d.System != 3 || d.IRQ != 1 || d.Idle != 10 {
		t.Fatalf("unexpected cpu delta fields: %+v", d)
	}

	// Total = 5+2+3+10+1 = 21; Used = Total - Idle = 11
	if d.Total != 21 {
		t.Fatalf("expected Total=21, got %d", d.Total)
	}
	if d.Used != 11 {
		t.Fatalf("expected Used=11, got %d", d.Used)
	}

	// Ensure FreeBSD-untracked fields are zeroed.
	if d.IOWait != 0 || d.SoftIRQ != 0 || d.Steal != 0 || d.Guest != 0 || d.GuestNice != 0 {
		t.Fatalf("expected untracked fields to be zero: %+v", d)
	}
}

func TestCalcCoreUsage_OnlyAggregate(t *testing.T) {
	dm := map[string]CPUDelta{
		"cpu": {Used: 50, Total: 100},
	}
	got := calcCoreUsage(dm)
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got len=%d (%v)", len(got), got)
	}
}

func TestCalcCoreUsage_TwoCores(t *testing.T) {
	// percent(Used,Total) should yield 25, 50.
	dm := map[string]CPUDelta{
		"cpu":  {Used: 75, Total: 100},
		"cpu0": {Used: 1, Total: 4},
		"cpu1": {Used: 1, Total: 2},
	}

	got := calcCoreUsage(dm)
	if len(got) != 2 {
		t.Fatalf("expected len=2, got len=%d (%v)", len(got), got)
	}

	assertApprox(t, got[0], 25.0, 1e-9)
	assertApprox(t, got[1], 50.0, 1e-9)
}

func TestCalcCoreUsage_MissingCoreEntryLeavesZero(t *testing.T) {
	dm := map[string]CPUDelta{
		"cpu":  {Used: 10, Total: 20},
		"cpu0": {Used: 1, Total: 2},
		// cpu1 missing
	}

	got := calcCoreUsage(dm)
	if len(got) != 1 { // len(map)-1 => 1
		t.Fatalf("expected len=1, got len=%d (%v)", len(got), got)
	}
	// Only cpu0 exists, so got[0] should be 50%.
	assertApprox(t, got[0], 50.0, 1e-9)
}

func assertApprox(t *testing.T, got, want, eps float64) {
	t.Helper()
	if math.IsNaN(got) || math.Abs(got-want) > eps {
		t.Fatalf("got %v want %v (eps %v)", got, want, eps)
	}
}
