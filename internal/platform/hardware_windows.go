//go:build windows

package platform

import (
	"strings"

	"golang.org/x/sys/windows/registry"
)

func isRaspberryPi() bool { return false }
func isContainer() bool   { return false }

func isVirtualMachine() bool {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `HARDWARE\DESCRIPTION\System\BIOS`, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()

	vmIndicators := []string{
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

	for _, name := range []string{"SystemProductName", "SystemManufacturer"} {
		val, _, err := k.GetStringValue(name)
		if err != nil {
			continue
		}
		for _, ind := range vmIndicators {
			if strings.Contains(strings.ToLower(val), ind) {
				return true
			}
		}
	}
	return false
}
