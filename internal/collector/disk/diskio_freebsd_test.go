//go:build freebsd

package disk

import (
	"testing"
	"unsafe"
)

// TestDevstatSize pins the struct against the kernel ABI it mirrors.
//
// Devstat is read straight out of kern.devstat.all, so its size and field
// offsets have to match sys/sys/devicestat.h exactly. Getting it wrong is
// silent: readDevstats rejects the buffer because the length is not a multiple
// of the struct size, disk_io returns an error every cycle, and the only trace
// is a DEBUG line. This host produced 576 bytes for two devices while the
// struct claimed 296, so no disk I/O was collected on FreeBSD at all.
//
// The trap was STAILQ_ENTRY, sized as a pointer pair. sys/queue.h gives
// SLIST_ENTRY and STAILQ_ENTRY one next pointer; only LIST_ENTRY and
// TAILQ_ENTRY add a back-pointer.
func TestDevstatSize(t *testing.T) {
	const want = 288 // amd64, DEVSTAT_VERSION 6

	if got := unsafe.Sizeof(Devstat{}); got != want {
		t.Errorf("sizeof(Devstat) = %d, want %d: the struct no longer matches the kernel ABI", got, want)
	}
}

// TestDevstatFieldOffsets catches a field that moved without changing the total
// size, which a size check alone would miss — swapping two adjacent fields of
// equal width, or trading padding for a real field.
func TestDevstatFieldOffsets(t *testing.T) {
	var d Devstat

	tests := []struct {
		name string
		got  uintptr
		want uintptr
	}{
		{"Sequence0", unsafe.Offsetof(d.Sequence0), 0},
		{"Allocated", unsafe.Offsetof(d.Allocated), 4},
		{"StartCount", unsafe.Offsetof(d.StartCount), 8},
		{"EndCount", unsafe.Offsetof(d.EndCount), 12},
		{"BusyFrom", unsafe.Offsetof(d.BusyFrom), 16},
		{"DeviceNumber", unsafe.Offsetof(d.DeviceNumber), 40},
		{"DeviceName", unsafe.Offsetof(d.DeviceName), 44},
		{"UnitNumber", unsafe.Offsetof(d.UnitNumber), 60},
		{"Bytes", unsafe.Offsetof(d.Bytes), 64},
		{"Operations", unsafe.Offsetof(d.Operations), 96},
		{"Duration", unsafe.Offsetof(d.Duration), 128},
		{"BusyTime", unsafe.Offsetof(d.BusyTime), 192},
		{"CreationTime", unsafe.Offsetof(d.CreationTime), 208},
		{"BlockSize", unsafe.Offsetof(d.BlockSize), 224},
		{"TagTypes", unsafe.Offsetof(d.TagTypes), 232},
		{"Flags", unsafe.Offsetof(d.Flags), 256},
		{"DeviceType", unsafe.Offsetof(d.DeviceType), 260},
		{"Priority", unsafe.Offsetof(d.Priority), 264},
		{"ID", unsafe.Offsetof(d.ID), 272},
		{"Sequence1", unsafe.Offsetof(d.Sequence1), 280},
	}

	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("offsetof(%s) = %d, want %d", tt.name, tt.got, tt.want)
		}
	}
}

// TestBinTimeSize guards the type every timing field in Devstat is built from,
// so a change there is reported once rather than as a cascade of offsets.
func TestBinTimeSize(t *testing.T) {
	if got := unsafe.Sizeof(BinTime{}); got != 16 {
		t.Errorf("sizeof(BinTime) = %d, want 16", got)
	}
}

func TestIsStorageDevice(t *testing.T) {
	// Values from sys/sys/devicestat.h devstat_type_flags.
	tests := []struct {
		name       string
		deviceType uint32
		want       bool
	}{
		{"nvme disk (direct, NVME interface)", DEVSTAT_TYPE_DIRECT | 0x040, true},
		{"scsi disk (direct, SCSI interface)", DEVSTAT_TYPE_DIRECT | 0x010, true},
		{"ide disk (direct, IDE interface)", DEVSTAT_TYPE_DIRECT | 0x020, true},
		{"bare direct device", DEVSTAT_TYPE_DIRECT, true},
		{"cam passthrough over a disk", DEVSTAT_TYPE_PASS | DEVSTAT_TYPE_DIRECT | 0x010, false},
		{"cdrom", 0x005, false},
		{"enclosure", 0x00d, false},
		{"sequential (tape)", 0x001, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isStorageDevice(tt.deviceType); got != tt.want {
				t.Errorf("isStorageDevice(%#03x) = %v, want %v", tt.deviceType, got, tt.want)
			}
		})
	}
}

// TestIsStorageDevice_PassthroughOverDirectIsExcluded pins the case that makes
// the mask ordering matter: a passthrough node reports the class of the device
// beneath it in the low bits, so checking DEVSTAT_TYPE_DIRECT without first
// rejecting DEVSTAT_TYPE_PASS would report pass0 alongside nda0 and double-count
// every operation.
func TestIsStorageDevice_PassthroughOverDirectIsExcluded(t *testing.T) {
	if isStorageDevice(DEVSTAT_TYPE_PASS | DEVSTAT_TYPE_DIRECT) {
		t.Error("passthrough device over a direct-access disk was reported as storage")
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
		ReadBytes:  1_000_000,
		WriteBytes: 2_000_000,
		ReadOps:    100,
		WriteOps:   200,
		ReadTime:   1_000,
		WriteTime:  2_000,
	}
	curr := IORaw{
		ReadBytes:  2_024_000, // +1,024,000 bytes
		WriteBytes: 4_560_000, // +2,560,000 bytes
		ReadOps:    140,       // +40
		WriteOps:   280,       // +80
		ReadTime:   3_000,     // +2000ms over a 10s interval -> 20%
		WriteTime:  8_000,     // +6000ms over a 10s interval -> 60%
		InProgress: 3,
	}

	m := buildDiskIOMetric("nda0", curr, prev, elapsed)

	if m.Device != "nda0" {
		t.Errorf("Device = %q, want %q", m.Device, "nda0")
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
		ReadBytes: 9_000_000, WriteBytes: 9_000_000,
		ReadOps: 5_000, WriteOps: 5_000,
		ReadTime: 90_000, WriteTime: 90_000,
	}
	curr := IORaw{
		ReadBytes: 10, WriteBytes: 20,
		ReadOps: 10, WriteOps: 20,
		ReadTime: 30, WriteTime: 40,
	}

	m := buildDiskIOMetric("nda0", curr, prev, 10.0)

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
