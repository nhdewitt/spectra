//go:build darwin

package memory

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"

	"github.com/nhdewitt/spectra/internal/protocol"
	"github.com/nhdewitt/spectra/internal/util"
	"golang.org/x/sys/unix"
)

// xswUsage mirrors the xsw_usage struct from sys/sysctl.h:
//
//	struct xsw_usage {
//			u_int64_t	xsu_total;
//			u_int64_t	xsu_avail;
//			u_int64_t	xsu_used;
//		};
type xswUsage struct {
	Total uint64
	Avail uint64
	Used  uint64
}

type memRaw struct {
	Total     uint64
	Available uint64
	SwapTotal uint64
	SwapFree  uint64
}

func Collect(ctx context.Context) ([]protocol.Metric, error) {
	raw, err := parseMemInfo()
	if err != nil {
		return nil, err
	}

	used := raw.Total - raw.Available
	swapUsed := raw.SwapTotal - raw.SwapFree

	return []protocol.Metric{protocol.MemoryMetric{
		Total:     raw.Total,
		Available: raw.Available,
		Used:      used,
		UsedPct:   util.Percent(used, raw.Total),
		SwapTotal: raw.SwapTotal,
		SwapUsed:  swapUsed,
		SwapPct:   util.Percent(swapUsed, raw.SwapTotal),
	}}, nil
}

func parseMemInfo() (memRaw, error) {
	total, err := sysctlInt("hw.memsize")
	if err != nil {
		return memRaw{}, fmt.Errorf("hw.memsize: %w", err)
	}

	pageSize, err := sysctlInt("hw.pagesize")
	if err != nil {
		return memRaw{}, fmt.Errorf("hw.pagesize: %w", err)
	}

	free, err := sysctlInt("vm.page_free_count")
	if err != nil {
		return memRaw{}, fmt.Errorf("vm.page_free_count: %w", err)
	}

	purgeable, err := sysctlInt("vm.page_purgeable_count")
	if err != nil {
		return memRaw{}, fmt.Errorf("vm.page_purgeable_count: %w", err)
	}

	available := (free + purgeable) * pageSize

	swap, err := parseSwapUsage()
	if err != nil {
		return memRaw{}, fmt.Errorf("vm.swapusage: %w", err)
	}

	return memRaw{
		Total:     total,
		Available: available,
		SwapTotal: swap.Total,
		SwapFree:  swap.Avail,
	}, nil
}

// sysctlInt reads in integer sysctl value, handling both 32 and 64-bit returns.
func sysctlInt(name string) (uint64, error) {
	b, err := unix.SysctlRaw(name)
	if err != nil {
		return 0, err
	}

	switch len(b) {
	case 4:
		return uint64(binary.LittleEndian.Uint32(b)), nil
	case 8:
		return binary.LittleEndian.Uint64(b), nil
	default:
		return 0, fmt.Errorf("unpexected size %d for %s", len(b), name)
	}
}

// parseSwapUsage reads vm.swapusage as a raw xsw_usage struct.
func parseSwapUsage() (xswUsage, error) {
	buf, err := unix.SysctlRaw("vm.swapusage")
	if err != nil {
		return xswUsage{}, err
	}

	var swap xswUsage
	if err := binary.Read(bytes.NewReader(buf), binary.LittleEndian, &swap); err != nil {
		return xswUsage{}, fmt.Errorf("parsing xsw_usage: %w", err)
	}

	return swap, nil
}
