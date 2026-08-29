//go:build linux

package platform

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// piModelPath is the device-tree node the Pi firmware populates.
var piModelPath = "/proc/device-tree/model"

func Detect() Info {
	info := Info{
		NumCPU: runtime.NumCPU(),
	}

	info.PackageManager, info.PackageExe = detectPackageManager()
	info.InitSystem, info.SystemctlPath = detectInitSystem()
	info.HasPSI = fileExists("/proc/pressure/memory")
	info.CgroupVersion = detectCgroupVersion()
	if data, err := os.ReadFile("/proc/sys/kernel/osrelease"); err == nil {
		info.KernelVersion = strings.TrimSpace(string(data))
	}
	info.IsRaspberryPi, info.PiModel = detectPi()
	if info.IsRaspberryPi {
		info.VcgencmdPath, _ = exec.LookPath("vcgencmd")
	}
	info.ThermalZones, _ = filepath.Glob("/sys/class/thermal/thermal_zone*")
	info.SmartctlPath, _ = exec.LookPath("smartctl")

	return info
}

func detectPackageManager() (PackageManager, string) {
	for _, check := range []struct {
		pm  PackageManager
		exe string
	}{
		{PkgApt, "apt"},
		{PkgYum, "yum"},
		{PkgApk, "apk"},
		{PkgPacman, "pacman"},
	} {
		if path, err := exec.LookPath(check.exe); err == nil {
			return check.pm, path
		}
	}
	return PkgNone, ""
}

func detectInitSystem() (InitSystem, string) {
	if path, err := exec.LookPath("systemctl"); err == nil {
		return InitSystemd, path
	}
	if fileExists("/sbin/openrc") {
		return InitOpenRC, ""
	}
	if fileExists("/etc/init.d") {
		return InitSysV, ""
	}
	return InitUnknown, ""
}

func detectCgroupVersion() int {
	if fileExists("/sys/fs/cgroup/cgroup.controllers") {
		return 2
	}
	if fileExists("/sys/fs/cgroup/cpu") {
		return 1
	}
	return 0
}

func detectPi() (bool, string) {
	data, err := os.ReadFile(piModelPath)
	if err != nil {
		return false, ""
	}
	return parsePiModel(data)
}

// parsePiModel reports whether a /proc/device-tree/model payload identifies a
// Raspberry Pi, and returns the trimmed model string when it does. The file is
// NUL-terminated by the device tree and may carry a trailing newline.
func parsePiModel(data []byte) (bool, string) {
	model := strings.TrimRight(string(data), "\x00\n")
	if strings.Contains(model, "Raspberry Pi") {
		return true, model
	}
	return false, ""
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
