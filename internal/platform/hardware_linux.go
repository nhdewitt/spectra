//go:build linux

package platform

import (
	"bytes"
	"os"
	"strings"
)

var hypervisors = []string{
	"vmware",
	"virtualbox",
	"kvm",
	"qemu",
	"xen",
	"microsoft corporation",
	"innotek gmbh",
	"bochs",
	"parallels",
	"amazon ec2",
	"google",
	"openstack",
	"ovirt",
	"nutanix",
}

// initCgroupPath is PID 1's cgroup membership.
var initCgroupPath = "/proc/1/cgroup"

func isRaspberryPi() bool {
	pi, _ := detectPi()
	return pi
}

func isContainer() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}

	if os.Getenv("container") != "" {
		return true
	}

	if b, err := os.ReadFile(initCgroupPath); err == nil {
		if cgroupIsContainer(string(b)) {
			return true
		}
	}
	return false
}

// cgroupIsContainer reports whether PID 1's cgroup membership names a known
// container runtime.
func cgroupIsContainer(s string) bool {
	for _, marker := range []string{"/docker/", "/lxc/", "/containerd/"} {
		if strings.Contains(s, marker) {
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

	for _, path := range candidates {
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if matchHypervisor(string(b), hypervisors) {
			return true
		}
	}
	return false
}

// matchHypervisor reports whether a DMI field names a known hypervisor. The
// comparison is case-insensitive because vendors are inconsistent.
func matchHypervisor(field string, hypervisors []string) bool {
	s := strings.ToLower(string(bytes.TrimSpace([]byte(field))))
	for _, h := range hypervisors {
		if strings.Contains(s, h) {
			return true
		}
	}
	return false
}
