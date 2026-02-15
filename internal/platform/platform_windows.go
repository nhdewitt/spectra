//go:build windows

package platform

import (
	"os/exec"
	"runtime"
)

func Detect() Info {
	return Info{
		PackageManager: PkgWindowsUpdate,
		InitSystem:     InitUnknown,
		NumCPU:         runtime.NumCPU(),
		PowerShellPath: findPowerShell(),
	}
}

func findPowerShell() string {
	if path, err := exec.LookPath("pwsh"); err == nil {
		return path
	}
	if path, err := exec.LookPath("powershell"); err == nil {
		return path
	}
	return ""
}
