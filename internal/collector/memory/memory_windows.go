//go:build windows
// +build windows

package memory

import (
	"context"
	"fmt"
	"unsafe"

	"github.com/nhdewitt/spectra/internal/protocol"
	"github.com/nhdewitt/spectra/internal/util"
	"github.com/nhdewitt/spectra/internal/winapi"
)

func Collect(ctx context.Context) ([]protocol.Metric, error) {
	var memStatus winapi.MemoryStatusEx
	memStatus.Length = uint32(unsafe.Sizeof(memStatus))

	ret, _, _ := winapi.ProcGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&memStatus)))
	if ret == 0 {
		return nil, fmt.Errorf("GlobalMemoryStatusEx failed")
	}

	usedPhys := memStatus.TotalPhys - memStatus.AvailPhys
	swapUsed := memStatus.TotalPageFile - memStatus.AvailPageFile

	result := protocol.MemoryMetric{
		Total:     memStatus.TotalPhys,
		Used:      usedPhys,
		Available: memStatus.AvailPhys,
		UsedPct:   util.Percent(usedPhys, memStatus.TotalPhys),
		SwapTotal: memStatus.TotalPageFile,
		SwapUsed:  swapUsed,
		SwapPct:   util.Percent(swapUsed, memStatus.TotalPageFile),
	}

	return []protocol.Metric{result}, nil
}
