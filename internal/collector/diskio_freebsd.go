//go:build freebsd

package collector

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
	"time"
	"unsafe"

	"github.com/nhdewitt/spectra/internal/protocol"
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

var (
	lastDiskIORaw  map[string]DiskIORaw
	lastDiskIOTime time.Time
)

// expectedDevstatSize is used at runtime to verify that the Go struct
// matches the kernel's devstat layout. If the struct changes,
// this check will fail instead of silently misparsing data.
var expectedDevstatSize = int(unsafe.Sizeof(Devstat{}))

type DiskIORaw struct {
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
//	    STAILQ_ENTRY(devstat) dev_links;   // pointer pair = 16 bytes on amd64
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
	_            [16]byte // STAILQ_ENTRY - two pointers
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

func MakeDiskIOCollector(cache *DriveCache) CollectFunc {
	return func(ctx context.Context) ([]protocol.Metric, error) {
		return CollectDiskIO(ctx, cache)
	}
}

func CollectDiskIO(ctx context.Context, cache *DriveCache) ([]protocol.Metric, error) {
	// Get list of devices
	mountMap := loadMountMap(cache)

	// Parse kernel stats
	currentRaw, err := getDevstats(mountMap)
	if err != nil {
		return nil, err
	}

	now := time.Now()

	// Baseline
	if len(lastDiskIORaw) == 0 {
		lastDiskIORaw = currentRaw
		lastDiskIOTime = now
		return nil, nil
	}

	elapsed := now.Sub(lastDiskIOTime).Seconds()
	if elapsed <= 0 {
		return nil, nil
	}

	result := make([]protocol.Metric, 0, len(currentRaw))

	for device, curr := range currentRaw {
		prev, ok := lastDiskIORaw[device]
		if !ok {
			continue
		}
		result = append(result, buildDiskIOMetric(device, curr, prev, elapsed))
	}

	lastDiskIORaw = currentRaw
	lastDiskIOTime = now

	return result, nil
}

func buildDiskIOMetric(device string, curr, prev DiskIORaw, elapsed float64) protocol.DiskIOMetric {
	return protocol.DiskIOMetric{
		Device:     device,
		ReadBytes:  uint64(float64(delta(curr.ReadBytes, prev.ReadBytes)) / elapsed),
		WriteBytes: uint64(float64(delta(curr.WriteBytes, prev.WriteBytes)) / elapsed),
		ReadOps:    rate(curr.ReadOps-prev.ReadOps, elapsed),
		WriteOps:   rate(curr.WriteOps-prev.WriteOps, elapsed),
		ReadTime:   delta(curr.ReadTime, prev.ReadTime),
		WriteTime:  delta(curr.WriteTime, prev.WriteTime),
		InProgress: curr.InProgress,
	}
}

// getDevstats retrieves the raw devstat data from kern.devstat.all.
func getDevstats(mountMap map[string]MountInfo) (map[string]DiskIORaw, error) {
	data, err := unix.SysctlRaw("kern.devstat.all")
	if err != nil {
		return nil, fmt.Errorf("sysctl kern.devstat.all: %w", err)
	}
	return parseDevStats(data, mountMap)
}

func parseDevStats(data []byte, mountMap map[string]MountInfo) (map[string]DiskIORaw, error) {
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

	monitored := make(map[string]struct{}, len(mountMap)*2)
	for _, m := range mountMap {
		monitored[m.Device] = struct{}{}
		monitored[strings.TrimPrefix(m.Device, "/dev/")] = struct{}{}
	}

	reader := bytes.NewReader(data)
	result := make(map[string]DiskIORaw, len(mountMap))

	for reader.Len() > 0 {
		var stat Devstat
		if err := binary.Read(reader, binary.LittleEndian, &stat); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("devstat parsing failed: %w", err)
		}

		name := unix.ByteSliceToString(stat.DeviceName[:])
		deviceKey := fmt.Sprintf("%s%d", name, stat.UnitNumber)

		if _, ok := monitored[deviceKey]; !ok {
			continue
		}

		// Busy count = start_count - end_count
		var inProgress uint64
		if stat.StartCount > stat.EndCount {
			inProgress = uint64(stat.StartCount - stat.EndCount)
		}

		result[deviceKey] = DiskIORaw{
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
