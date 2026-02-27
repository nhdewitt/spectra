//go:build darwin

package platform

import (
	"os/exec"
	"runtime"
	"strings"

	"golang.org/x/sys/unix"
)

func Detect() Info {
	info := Info{
		NumCPU:     runtime.NumCPU(),
		InitSystem: InitLaunchd,
	}

	info.PackageManager, info.PackageExe = detectPackageManager()

	if rel, err := unix.Sysctl("kern.osrelease"); err == nil {
		info.KernelVersion = strings.TrimSpace(rel)
	}

	info.SmartctlPath, _ = exec.LookPath("smartctl")
	info.SystemctlPath = "launchctl"

	return info
}

func detectPackageManager() (PackageManager, string) {
	if path, err := exec.LookPath("brew"); err == nil {
		return PkgBrew, path
	}
	return PkgNone, ""
}
