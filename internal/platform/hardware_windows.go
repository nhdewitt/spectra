//go:build windows

package platform

import (
	"strings"

	"golang.org/x/sys/windows/registry"
)

var vmIndicators = []string{
	"vmware virtual platform",
	"vmware7,",
	"virtualbox",
	"innotek gmbh",
	"bochs",
	"qemu",
	"kvm",
	"xen",
	"hyper-v",
	"virtual machine",
	"parallels",
	"amazon ec2",
	"google compute engine",
	"openstack",
	"ovirt",
	"bhyve",
	"nutanix",
}

func isRaspberryPi() bool { return false }
func isContainer() bool   { return false }

func isVirtualMachine() bool {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `HARDWARE\DESCRIPTION\System\BIOS`, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()

	for _, name := range []string{"SystemProductName", "SystemManufacturer"} {
		val, _, err := k.GetStringValue(name)
		if err != nil {
			continue
		}
		if matchVMIndicator(val, vmIndicators) {
			return true
		}
	}
	return false
}

// matchVMIndicator reports whether a BIOS registry value names a hypervisor.
// Case-insensitive because vendors are inconsistent.
func matchVMIndicator(val string, indicators []string) bool {
	lower := strings.ToLower(val)
	for _, ind := range indicators {
		if strings.Contains(lower, ind) {
			return true
		}
	}
	return false
}
