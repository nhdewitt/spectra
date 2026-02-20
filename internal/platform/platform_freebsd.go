//go:build freebsd

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
		InitSystem: InitRC,
	}

	info.PackageManager, info.PackageExe = detectPackageManager()

	if rel, err := unix.Sysctl("kern.osrelease"); err == nil {
		info.KernelVersion = strings.TrimSpace(rel)
	}

	info.SmartctlPath, _ = exec.LookPath("smartctl")

	return info
}

func detectPackageManager() (PackageManager, string) {
	if path, err := exec.LookPath("pkg"); err == nil {
		return PkgPkg, path
	}
	return PkgNone, ""
}
