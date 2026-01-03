//go:build windows
// +build windows

package collector

import (
	"context"
	"fmt"
	"unsafe"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func CollectMemory(ctx context.Context) ([]protocol.Metric, error) {
	var memStatus memoryStatusEx
	memStatus.Length = uint32(unsafe.Sizeof(memStatus))

	ret, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&memStatus)))
	if ret == 0 {
		return nil, fmt.Errorf("GlobalMemoryStatusEx failed")
	}

	usedPhys := memStatus.TotalPhys - memStatus.AvailPhys
	swapUsed := memStatus.TotalPageFile - memStatus.AvailPageFile

	result := protocol.MemoryMetric{
		Total:     memStatus.TotalPhys,
		Used:      usedPhys,
		Available: memStatus.AvailPhys,
		UsedPct:   percent(usedPhys, memStatus.TotalPhys),
		SwapTotal: memStatus.TotalPageFile,
		SwapUsed:  swapUsed,
		SwapPct:   percent(swapUsed, memStatus.TotalPageFile),
	}

	return []protocol.Metric{result}, nil
}
