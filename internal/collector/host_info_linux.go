//go:build !windows

package collector

import (
	"bufio"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"
)

func getPlatformInfo() (string, string) {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return "linux", ""
	}
	defer f.Close()

	return getPlatformInfoFrom(f)
}

func getPlatformInfoFrom(r io.Reader) (platform, version string) {
	platform = "linux"
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "ID=") {
			platform = strings.Trim(line[3:], `"`)
		} else if strings.HasPrefix(line, "VERSION_ID=") {
			version = strings.Trim(line[11:], `"`)
		}

		if platform != "linux" && version != "" {
			break
		}
	}

	platform = strings.TrimSpace(platform)
	version = strings.TrimSpace(version)

	if platform == "" {
		platform = "linux"
	}

	return platform, version
}

func getKernel() string {
	var uname syscall.Utsname
	if err := syscall.Uname(&uname); err != nil {
		return ""
	}

	return charsToString(uname.Release[:])
}

// charsToString converts a NUL-terminated C char buffer to a Go string.
// It accepts both signed and unsigned byte representations.
func charsToString[T ~int8 | ~uint8](ca []T) string {
	buf := make([]byte, 0, len(ca))

	for _, c := range ca {
		if c == 0 {
			break
		}
		buf = append(buf, byte(c))
	}

	return string(buf)
}

func getCPUModel() string {
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return ""
	}
	defer f.Close()

	return getCPUModelFrom(f)
}

func getCPUModelFrom(r io.Reader) string {
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "model name") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}

	return ""
}

func getRAMTotal() uint64 {
	if v := MemTotal(); v > 0 {
		return v
	}

	raw, err := parseMemInfo()
	if err != nil {
		return 0
	}
	cachedMemTotal.Store(raw.Total)
	return raw.Total
}

func getBootTime() int64 {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0
	}
	defer f.Close()

	return getBootTimeFrom(f)
}

func getBootTimeFrom(r io.Reader) int64 {
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "btime ") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				btime, err := strconv.ParseInt(fields[1], 10, 64)
				if err != nil {
					return 0
				}
				return btime
			}
		}
	}

	return 0
}
