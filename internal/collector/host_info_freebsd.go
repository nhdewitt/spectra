//go:build freebsd

package collector

import (
	"bufio"
	"io"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

func getPlatformInfo() (string, string) {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return "freebsd", getKernel()
	}
	defer f.Close()

	return getPlatformInfoFrom(f)
}

func getPlatformInfoFrom(r io.Reader) (platform, version string) {
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "ID=") {
			platform = strings.Trim(line[3:], `"`)
		} else if strings.HasPrefix(line, "VERSION_ID=") {
			version = strings.Trim(line[11:], `"`)
		}
	}

	if platform == "" {
		platform = "freebsd"
	}

	return platform, version
}

func getKernel() string {
	var uname unix.Utsname
	if err := unix.Uname(&uname); err != nil {
		return ""
	}

	return charsToString(uname.Release[:])
}

// getCPUModel reads hw.model via sysctl.
func getCPUModel() string {
	model, err := unix.Sysctl("hw.model")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(model)
}

// getRAMTotal reads hw.physmem via sysctl
func getRAMTotal() uint64 {
	mem, err := unix.SysctlUint64("hw.physmem")
	if err != nil {
		return 0
	}
	return mem
}

// getBootTime reads kern.boottime via sysctl.
func getBootTime() int64 {
	tv, err := unix.SysctlTimeval("kern.boottime")
	if err != nil {
		return 0
	}
	return int64(tv.Sec)
}
