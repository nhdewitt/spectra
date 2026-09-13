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
