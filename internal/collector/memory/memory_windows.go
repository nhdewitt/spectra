//go:build windows

package memory

import (
	"context"
	"fmt"
	"unsafe"

	"github.com/nhdewitt/spectra/internal/protocol"
	"github.com/nhdewitt/spectra/internal/util"
	"github.com/nhdewitt/spectra/internal/winapi"
	"github.com/yusufpapurcu/wmi"
)

// Win32_PageFileUsage maps to the WMI class. The tags tell the libraryt which
// WMI properties to load. Sizes are reported in MB.
type Win32_PageFileUsage struct {
	AllocatedBaseSize uint32
	CurrentUsage      uint32
	Name              string
}

const bytesPerMB = 1024 * 1024

// pageFileUsage returns total and in-use pagefile bytes, summed across every
// configured pagefile.
//
// A host with no pagefile is a supported configuration. The query suceeds with
// zero rows and (0, 0, nil) which matches what a Linux host with no swap
// device reports.
func pageFileUsage() (total, used uint64, err error) {
	var dst []Win32_PageFileUsage

	q := wmi.CreateQuery(&dst, "")
	if err := wmi.Query(q, &dst); err != nil {
		return 0, 0, fmt.Errorf("querying Win32_PageFileUsage: %w", err)
	}

	total, used = sumPageFiles(dst)
	return total, used, nil
}

// sumPageFiles converts WMI's MB figures to bytes and sums them across every
// configured pagefile. Split from the query for testing without a live WMI.
func sumPageFiles(rows []Win32_PageFileUsage) (total, used uint64) {
	for _, pf := range rows {
		total += uint64(pf.AllocatedBaseSize) * bytesPerMB
		used += uint64(pf.CurrentUsage) * bytesPerMB
	}
	return
}

func Collect(ctx context.Context) ([]protocol.Metric, error) {
	var memStatus winapi.MemoryStatusEx
	memStatus.Length = uint32(unsafe.Sizeof(memStatus))

	ret, _, _ := winapi.ProcGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&memStatus)))
	if ret == 0 {
		return nil, fmt.Errorf("GlobalMemoryStatusEx failed")
	}

	usedPhys := memStatus.TotalPhys - memStatus.AvailPhys

	// ullTotalPageFile is the system commit limit and ullAvailPageFile the remaining
	// commit, so this pair is commit accounting, not pagefile usage. Commit exhaustion
	// is what causes allocation failures on Windows, so it is reported in its own columns
	// alongside the real pagefile figures. Linux reports the same pair as CommitLimit
	// and Committed_AS.
	commitLimit := memStatus.TotalPageFile
	commitUsed := memStatus.TotalPageFile - memStatus.AvailPageFile

	// MEMORYSTATUSEX reports the system commit limit and the remaining commit,
	// not pagefile size and pagefile usage. Commit charge is dominated by resident
	// private memory, so deriving swap from it shows a busy pagefile on a host that
	// has never paged and is not comparable to swap_used on Linux/FreeBSD.
	swapTotal, swapUsed, swapErr := pageFileUsage()

	result := protocol.MemoryMetric{
		Total:     memStatus.TotalPhys,
		Used:      usedPhys,
		Available: memStatus.AvailPhys,
		UsedPct:   util.Percent(usedPhys, memStatus.TotalPhys),
		SwapTotal: swapTotal,
		SwapUsed:  swapUsed,
		SwapPct:   util.Percent(swapUsed, swapTotal),

		CommitLimit: &commitLimit,
		CommitUsed:  &commitUsed,
	}

	return []protocol.Metric{result}, swapErr
}
