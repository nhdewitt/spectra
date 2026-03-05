//go:build darwin

package collector

import (
	"strings"

	"golang.org/x/sys/unix"
)

func getPlatformInfo() (string, string) {
	version, err := unix.Sysctl("kern.osproductversion")
	if err != nil {
		return "macOS", ""
	}
	return "macOS", strings.TrimSpace(version)
}

func getKernel() string {
	rel, err := unix.Sysctl("kern.osrelease")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(rel)
}

func getCPUModel() string {
	model, err := unix.Sysctl("machdep.cpu.brand_string")
	if err != nil {
		// fallback to hw.model (e.g. MacBookAir10,1)
		hw, _ := unix.Sysctl("hw.model")
		return strings.TrimSpace(hw)
	}
	return strings.TrimSpace(model)
}

func getRAMTotal() uint64 {
	mem, err := unix.SysctlUint64("hw.memsize")
	if err != nil {
		return 0
	}
	return mem
}

func getBootTime() int64 {
	tv, err := unix.SysctlTimeval("kern.boottime")
	if err != nil {
		return 0
	}
	return tv.Sec
}
