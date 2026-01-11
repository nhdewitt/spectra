//go:build windows

package collector

import (
	"time"
	"unsafe"

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
	var mem memoryStatusEx
	mem.Length = uint32(unsafe.Sizeof(mem))
	procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&mem)))
	return mem.TotalPhys
}

func getBootTime() int64 {
	ret, _, _ := procGetTickCount64.Call()
	uptime := time.Duration(ret) * time.Millisecond
	return time.Now().Add(-uptime).Unix()
}
