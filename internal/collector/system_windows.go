//go:build windows

package collector

import (
	"context"
	"os/exec"
	"strings"
	"time"
	"unsafe"

	"github.com/nhdewitt/spectra/internal/protocol"
	"golang.org/x/sys/windows"
)

func CollectSystem(ctx context.Context) ([]protocol.Metric, error) {
	// Uptime
	ret, _, _ := procGetTickCount64.Call()
	uptimeSeconds := uint64(ret) / 1000

	bootTime := uint64(time.Now().Unix()) - uptimeSeconds

	// Process Count - Snapshot
	handle, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	processCount := 0
	if err == nil {
		defer windows.CloseHandle(handle)

		var pe32 windows.ProcessEntry32
		pe32.Size = uint32(unsafe.Sizeof(pe32))

		if err := windows.Process32First(handle, &pe32); err == nil {
			processCount++
			for {
				if err := windows.Process32Next(handle, &pe32); err != nil {
					// No more processes
					break
				}
				processCount++
			}
		}
	}

	// User Count - `quser`
	users := 0
	out, err := exec.CommandContext(ctx, "quser").Output()
	if err == nil {
		users = countQUserLines(out)
	}

	return []protocol.Metric{
		protocol.SystemMetric{
			Uptime:    uptimeSeconds,
			BootTime:  bootTime,
			Processes: processCount,
			Users:     users,
		},
	}, nil
}

// countQUserLines parses the output of `quser`.
func countQUserLines(out []byte) int {
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")

	if len(lines) <= 1 {
		return 0
	}

	count := 0
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}
