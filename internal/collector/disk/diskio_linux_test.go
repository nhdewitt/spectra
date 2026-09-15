//go:build linux
// +build linux

package disk

import (
	"context"
	"testing"
)

func BenchmarkCollectDiskIO(b *testing.B) {
	ctx := context.Background()
	mountCache := setupMountCache(b)

	diskIOCollector := MakeDiskIOCollector(mountCache)
	b.ResetTimer()

	for b.Loop() {
		diskIOCollector(ctx)
	}
}

// TestBuildDiskIOMetric_ReadAndWriteAreIndependent pins every derived field to a
// distinct value.
//
// writeBusy was computed from readTimeDelta, so write_busy_pct was a copy of
// read_busy_pct on every sample this collector has ever produced. Nothing
// caught it: writeTimeDelta was still consumed by the WriteTime field, so there
// was no unused variable, and the value it produced was always plausible.
//
// The fixture therefore makes read and write differ on every counter, and no
// two derived values coincide - a test where readTime equals writeTime, or
// where the busy and latency figures happen to match, cannot distinguish a
// correct derivation from one wired to the wrong delta.
func TestBuildDiskIOMetric_ReadAndWriteAreIndependent(t *testing.T) {
	const elapsed = 10.0

	prev := IORaw{
		ReadSectors:  1_000,
		WriteSectors: 2_000,
		ReadOps:      100,
		WriteOps:     200,
		ReadTime:     1_000,
		WriteTime:    2_000,
	}
	curr := IORaw{
		ReadSectors:  3_000, // +2000 sectors = 1,024,000 bytes
		WriteSectors: 7_000, // +5000 sectors = 2,560,000 bytes
		ReadOps:      140,   // +40
		WriteOps:     280,   // +80
		ReadTime:     3_000, // +2000ms over a 10s interval -> 20%
		WriteTime:    8_000, // +6000ms over a 10s interval -> 60%
		InProgress:   3,
	}

	m := buildDiskIOMetric("sda", curr, prev, elapsed)

	if m.Device != "sda" {
		t.Errorf("Device = %q, want %q", m.Device, "sda")
	}

	if m.ReadTime != 2_000 {
		t.Errorf("ReadTime = %d, want 2000", m.ReadTime)
	}
	if m.WriteTime != 6_000 {
		t.Errorf("WriteTime = %d, want 6000", m.WriteTime)
	}

	if m.ReadBusyPct == nil || m.WriteBusyPct == nil {
		t.Fatal("busy percentages are nil")
	}
	if got := *m.ReadBusyPct; got != 20 {
		t.Errorf("ReadBusyPct = %v, want 20", got)
	}
	if got := *m.WriteBusyPct; got != 60 {
		t.Errorf("WriteBusyPct = %v, want 60 (it was derived from the read delta)", got)
	}
	if *m.ReadBusyPct == *m.WriteBusyPct {
		t.Error("read and write busy are equal; one is derived from the other's delta")
	}

	if m.ReadLatency == nil || m.WriteLatency == nil {
		t.Fatal("latencies are nil")
	}
	if got := *m.ReadLatency; got != 50 {
		t.Errorf("ReadLatency = %v, want 50 (2000ms / 40 ops)", got)
	}
	if got := *m.WriteLatency; got != 75 {
		t.Errorf("WriteLatency = %v, want 75 (6000ms / 80 ops)", got)
	}

	if m.ReadOps != 4 {
		t.Errorf("ReadOps = %v, want 4/s", m.ReadOps)
	}
	if m.WriteOps != 8 {
		t.Errorf("WriteOps = %v, want 8/s", m.WriteOps)
	}

	if m.ReadBytes != 102_400 {
		t.Errorf("ReadBytes = %d, want 102_400", m.ReadBytes)
	}
	if m.WriteBytes != 256_000 {
		t.Errorf("WriteBytes = %d, want 256_000", m.WriteBytes)
	}

	if m.InProgress != 3 {
		t.Errorf("InProgress = %d, want 3", m.InProgress)
	}
}

// TestBuildDiskIOMetric_CounterResetClampsToZero covers the other half of why
// these deltas are taken through util.Delta: a device that disappears and comes
// back reports counters below the previous sample, and an unclamped subtraction
// would wrap to an enormous uint64.
func TestBuildDiskIOMetric_CounterResetClampsToZero(t *testing.T) {
	prev := IORaw{
		ReadSectors: 9_000, WriteSectors: 9_000,
		ReadOps: 5_000, WriteOps: 5_000,
		ReadTime: 90_000, WriteTime: 90_000,
	}
	curr := IORaw{
		ReadSectors: 10, WriteSectors: 20,
		ReadOps: 10, WriteOps: 20,
		ReadTime: 30, WriteTime: 40,
	}

	m := buildDiskIOMetric("sda", curr, prev, 10.0)

	if m.ReadTime != 0 || m.WriteTime != 0 {
		t.Errorf("times = %d/%d, want 0/0 after a counter reset", m.ReadTime, m.WriteTime)
	}
	if *m.ReadBusyPct != 0 || *m.WriteBusyPct != 0 {
		t.Errorf("busy = %v/%v, want 0/0 after a counter reset", *m.ReadBusyPct, *m.WriteBusyPct)
	}
	if m.ReadBytes != 0 || m.WriteBytes != 0 {
		t.Errorf("bytes = %d/%d, want 0/0 after a counter reset", m.ReadBytes, m.WriteBytes)
	}
}
