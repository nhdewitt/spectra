//go:build darwin

package platform

import (
	"os/exec"
	"strings"
)

func isRaspberryPi() bool { return false }
func isContainer() bool   { return false }

func isVirtualMachine() bool {
	// kern.hv_vmm_present (macOS 10.10+) returns 1 if running under a hypervisor
	out, err := exec.Command("sysctl", "-n", "kern.hv_vmm_present").Output()
	if err == nil && strings.TrimSpace(string(out)) == "1" {
		return true
	}

	// fallback: machdep.cpu.features contains "VMM" when virtualized
	out, err = exec.Command("sysctl", "-n", "machdep.cpu.features").Output()
	if err == nil && strings.Contains(string(out), "VMM") {
		return true
	}
	return false
}
