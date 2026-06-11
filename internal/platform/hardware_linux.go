//go:build linux

package platform

import (
	"bytes"
	"os"
	"strings"
)

func isRaspberryPi() bool {
	b, err := os.ReadFile("/proc/device-tree/model")
	if err != nil {
		return false
	}

	return strings.Contains(string(bytes.TrimRight(b, "\x00")), "Raspberry Pi")
}

func isContainer() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}

	if os.Getenv("container") != "" {
		return true
	}

	if b, err := os.ReadFile("/proc/1/cgroup"); err == nil {
		s := string(b)
		if strings.Contains(s, "/docker/") || strings.Contains(s, "/lxc/") || strings.Contains(s, "/containerd/") {
			return true
		}
	}
	return false
}

func isVirtualMachine() bool {
	candidates := []string{
		"/sys/class/dmi/id/product_name",
		"/sys/class/dmi/id/sys_vendor",
		"/sys/class/dmi/id/bios_vendor",
	}
	hypervisors := []string{
		"vmware", "virtualbox", "kvm", "qemu", "xen", "microsoft corporation", "innotek gmbh", "bochs",
		"parallels", "amazon ec2", "google", "openstack", "ovirt", "nutanix",
	}
	for _, path := range candidates {
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		s := strings.ToLower(string(bytes.TrimSpace(b)))
		for _, h := range hypervisors {
			if strings.Contains(s, h) {
				return true
			}
		}
	}
	return false
}
