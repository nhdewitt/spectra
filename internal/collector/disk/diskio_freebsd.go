//go:build freebsd

package disk

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"time"
	"unsafe"

	"github.com/nhdewitt/spectra/internal/collector"
	"github.com/nhdewitt/spectra/internal/protocol"
	"github.com/nhdewitt/spectra/internal/util"
	"golang.org/x/sys/unix"
)

// FreeBSD devstat indices
const (
	DEVSTAT_NO_DATA = 0
	DEVSTAT_READ    = 1
	DEVSTAT_WRITE   = 2
	DEVSTAT_FREE    = 3

	DEVSTAT_N_TRANS_FLAGS = 4
	DEVSTAT_NAME_LEN      = 16
)

// devstat_type_flags, from sys/sys/devicestat.h. The low four bits are
// the device class. 0x100 marks a passthrough device, whose lower bits
// then describe the physical device underneath it.
const (
	DEVSTAT_TYPE_MASK   = 0x00f
	DEVSTAT_TYPE_DIRECT = 0x000
	DEVSTAT_TYPE_PASS   = 0x100
)

var (
	lastIORaw  map[string]IORaw
	lastIOTime time.Time
)

// Standard C long is 8 bytes on amd64/arm64
const longSize = 8

// expectedDevstatSize is used at runtime to verify that the Go struct
// matches the kernel's devstat layout. If the struct changes,
// this check will fail instead of silently misparsing data.
var expectedDevstatSize = int(unsafe.Sizeof(Devstat{}))

type IORaw struct {
	DeviceName string
	ReadBytes  uint64
	WriteBytes uint64
	ReadTime   uint64 // ms
	WriteTime  uint64 // ms
	ReadOps    uint64
	WriteOps   uint64
	InProgress uint64
}

type BinTime struct {
	Sec  int64  // time_t
	Frac uint64 // 2^64ths of a second
}

// Devstat mirrors FreeBSD's struct devstat
//
// Source: sys/sys/devicestat.h
//
//	struct devstat {
//	    u_int           sequence0;
//	    int             allocated;
//	    u_int           start_count;
//	    u_int           end_count;
//	    struct bintime  busy_from;
//	    STAILQ_ENTRY(devstat) dev_links;   // single pointer (8 bytes on amd64)
//	    u_int32_t       device_number;
//	    char            device_name[DEVSTAT_NAME_LEN];
//	    int             unit_number;
//	    u_int64_t       bytes[DEVSTAT_N_TRANS_FLAGS];
//	    u_int64_t       operations[DEVSTAT_N_TRANS_FLAGS];
//	    struct bintime  duration[DEVSTAT_N_TRANS_FLAGS];
//	    struct bintime  busy_time;
//	    struct bintime  creation_time;
//	    u_int32_t       block_size;
//	    u_int64_t       tag_types[3];
//	    devstat_support_flags flags;        // enum = u_int
//	    devstat_type_flags    device_type;  // enum = u_int
//	    devstat_priority      priority;     // enum = u_int
//	    const void           *id;           // 8 bytes on amd64
//	    u_int           sequence1;
//	};
type Devstat struct {
	Sequence0    uint32
	Allocated    int32
	StartCount   uint32
	EndCount     uint32
	BusyFrom     BinTime
	_            [8]byte // STAILQ_ENTRY - single pointer
	DeviceNumber uint32
	DeviceName   [DEVSTAT_NAME_LEN]byte
	UnitNumber   int32
	Bytes        [DEVSTAT_N_TRANS_FLAGS]uint64
	Operations   [DEVSTAT_N_TRANS_FLAGS]uint64
	Duration     [DEVSTAT_N_TRANS_FLAGS]BinTime
	BusyTime     BinTime
	CreationTime BinTime
	BlockSize    uint32
	_            [4]byte // padding before u64 array
	TagTypes     [3]uint64
	Flags        uint32
	DeviceType   uint32
	Priority     uint32
	_            [4]byte // padding
	ID           uint64
	Sequence1    uint32
	_            [4]byte // padding
}

// MakeDiskIOCollector keeps the DriveCache parameter for signature parity with
// the other platforms, but freebsd no longer consults it. devstat identifies
// storage devices by class, so disk I/O does not depend on the mount manager.
func MakeDiskIOCollector(cache *DriveCache) collector.CollectFunc {
	return CollectDiskIO
}

func CollectDiskIO(ctx context.Context) ([]protocol.Metric, error) {
	// Parse kernel stats. No mount lookup. devstat identifies storage devices
	// by class, and tying this to the mount table broke ZFS hosts.
	currentIORaw, err := getDevstats()
	if err != nil {
		return nil, err
	}

	now := time.Now()

	// Baseline
	if len(lastIORaw) == 0 {
		lastIORaw = currentIORaw
		lastIOTime = now
		return nil, nil
	}

	elapsed := now.Sub(lastIOTime).Seconds()
	if elapsed <= 0 {
		return nil, nil
	}

	result := make([]protocol.Metric, 0, len(currentIORaw))

	for device, curr := range currentIORaw {
		prev, ok := lastIORaw[device]
		if !ok {
			continue
		}
		result = append(result, buildDiskIOMetric(device, curr, prev, elapsed))
	}

	lastIORaw = currentIORaw
	lastIOTime = now

	return result, nil
}

func buildDiskIOMetric(device string, curr, prev IORaw, elapsed float64) protocol.DiskIOMetric {
	// see diskio_linux.go's buildDiskIOMetric
	readOpsDelta := util.Delta(curr.ReadOps, prev.ReadOps)
	writeOpsDelta := util.Delta(curr.WriteOps, prev.WriteOps)
	readTimeDelta := util.Delta(curr.ReadTime, prev.ReadTime)
	writeTimeDelta := util.Delta(curr.WriteTime, prev.WriteTime)

	readLatency := AwaitMs(readTimeDelta, readOpsDelta)
	writeLatency := AwaitMs(writeTimeDelta, writeOpsDelta)
	readBusy := BusyPct(readTimeDelta, elapsed)
	writeBusy := BusyPct(writeTimeDelta, elapsed)

	return protocol.DiskIOMetric{
		Device:     device,
		ReadBytes:  uint64(float64(util.Delta(curr.ReadBytes, prev.ReadBytes)) / elapsed),
		WriteBytes: uint64(float64(util.Delta(curr.WriteBytes, prev.WriteBytes)) / elapsed),
		ReadOps:    util.Rate(readOpsDelta, elapsed),
		WriteOps:   util.Rate(writeOpsDelta, elapsed),

		ReadTime:  readTimeDelta,
		WriteTime: writeTimeDelta,

		ReadLatency:  &readLatency,
		WriteLatency: &writeLatency,
		ReadBusyPct:  &readBusy,
		WriteBusyPct: &writeBusy,

		InProgress: curr.InProgress,
	}
}

// getDevstats retrieves the IORaw devstat data from kern.devstat.all.
func getDevstats() (map[string]IORaw, error) {
	data, err := unix.SysctlRaw("kern.devstat.all")
	if err != nil {
		return nil, fmt.Errorf("sysctl kern.devstat.all: %w", err)
	}
	return parseDevStats(data)
}

// isStorageDevice reports whether a devstat entry is a real disk worth reporting
// as opposed to a CAM passthrough node or a CDROM.
//
// This replaces matching device names against mounted filesystems, which only worked
// when a mount's device string was also a devstat device name. That holds for UFS on
// a plain partition but fails completely for ZFS.
func isStorageDevice(deviceType uint32) bool {
	if deviceType&DEVSTAT_TYPE_PASS != 0 {
		return false
	}
	return deviceType&DEVSTAT_TYPE_MASK == DEVSTAT_TYPE_DIRECT
}

func parseDevStats(data []byte) (map[string]IORaw, error) {
	// kern.devstat.all is prefixed with a uint64 generation number
	// skip it before parsing the struct
	if len(data) < longSize {
		return nil, fmt.Errorf("devstat too short: %d bytes", len(data))
	}
	data = data[longSize:]

	if len(data) > 0 && len(data)%expectedDevstatSize != 0 {
		return nil, fmt.Errorf(
			"devstat data length %d is not a multiple of "+
				"expected struct size %d; possible FreeBSD DEVSTAT_VERSION mismatch",
			len(data), expectedDevstatSize,
		)
	}

	reader := bytes.NewReader(data)
	result := make(map[string]IORaw)

	for reader.Len() > 0 {
		var stat Devstat
		if err := binary.Read(reader, binary.LittleEndian, &stat); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("devstat parsing failed: %w", err)
		}

		if !isStorageDevice(stat.DeviceType) {
			continue
		}

		name := unix.ByteSliceToString(stat.DeviceName[:])
		deviceKey := fmt.Sprintf("%s%d", name, stat.UnitNumber)

		// Busy count = start_count - end_count
		var inProgress uint64
		if stat.StartCount > stat.EndCount {
			inProgress = uint64(stat.StartCount - stat.EndCount)
		}

		result[deviceKey] = IORaw{
			DeviceName: deviceKey,
			ReadOps:    stat.Operations[DEVSTAT_READ],
			WriteOps:   stat.Operations[DEVSTAT_WRITE],
			ReadBytes:  stat.Bytes[DEVSTAT_READ],
			WriteBytes: stat.Bytes[DEVSTAT_WRITE],
			ReadTime:   bintimeToMs(stat.Duration[DEVSTAT_READ]),
			WriteTime:  bintimeToMs(stat.Duration[DEVSTAT_WRITE]),
			InProgress: inProgress,
		}
	}

	return result, nil
}

// bintimeToMs converts FreeBSD bintime to milliseconds
func bintimeToMs(bt BinTime) uint64 {
	if bt.Sec < 0 {
		return 0
	}

	ms := uint64(bt.Sec) * 1000
	fracMs := (float64(bt.Frac) / float64(math.MaxUint64)) * 1000.0

	return ms + uint64(fracMs)
}
