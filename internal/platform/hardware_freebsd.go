//go:build freebsd

package platform

import (
	"os/exec"
	"strings"
)

func isRaspberryPi() bool { return false }
func isContainer() bool   { return false }

func isVirtualMachine() bool {
	// kern.vm_guest: none on bare metal/hypervisor name when virtualized
	out, err := exec.Command("sysctl", "-n", "kern.vm_guest").Output()
	if err != nil {
		return false
	}
	val := strings.TrimSpace(string(out))
	return val != "" && val != "none"
}
