//go:build windows

package hostinfo

import (
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/nhdewitt/spectra/internal/winapi"
	"golang.org/x/sys/windows/registry"
)

func getPlatformInfo() (platform, version string) {
	platform = "windows"
	k, err := registry.OpenKey(registry.LOCAL_MACHINE,
		`SOFTWARE\Microsoft\Windows NT\CurrentVersion`, registry.QUERY_VALUE)
	if err != nil {
		return "windows", ""
	}
	defer k.Close()

	if val, _, err := k.GetStringValue("ProductName"); err == nil {
		platform = val
		if strings.Contains(platform, "Windows 10") {
			if build, _, err := k.GetStringValue("CurrentBuildNumber"); err == nil {
				if n, _ := strconv.Atoi(build); n >= 22000 {
					platform = strings.Replace(platform, "Windows 10", "Windows 11", 1)
				}
			}
		}
	}

	if val, _, err := k.GetStringValue("DisplayVersion"); err == nil {
		version = val
	}

	return platform, version
}

func getKernel() (build string) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE,
		`SOFTWARE\Microsoft\Windows NT\CurrentVersion`, registry.QUERY_VALUE)
	if err != nil {
		return build
	}
	defer k.Close()

	build, _, _ = k.GetStringValue("CurrentBuildNumber")

	return build
}

func getCPUModel() (cpu string) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE,
		`HARDWARE\DESCRIPTION\System\CentralProcessor\0`, registry.QUERY_VALUE)
	if err != nil {
		return cpu
	}
	defer k.Close()

	cpu, _, _ = k.GetStringValue("ProcessorNameString")

	return cpu
}

func getRAMTotal() uint64 {
	var mem winapi.MemoryStatusEx
	mem.Length = uint32(unsafe.Sizeof(mem))
	winapi.ProcGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&mem)))
	return mem.TotalPhys
}

func getBootTime() int64 {
	ret, _, _ := winapi.ProcGetTickCount64.Call()
	uptime := time.Duration(ret) * time.Millisecond
	return time.Now().Add(-uptime).Unix()
}
