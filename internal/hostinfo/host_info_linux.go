//go:build linux

package hostinfo

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/nhdewitt/spectra/internal/collector/memory"
	"github.com/nhdewitt/spectra/internal/util"
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

	return util.CharsToString(uname.Release[:])
}

// getCPUModel returns a human-readable CPU model string. It first attempts to
// read the "model name" field from /proc/cpuinfo, which is present on x86/x64 and
// some 32-bit ARM kernels. If that field is absent (common on 64-bit ARM), it
// falls back to parsing lscpu output, appending the board model
// e.g. Cortex-A72 (Raspberry Pi 4 Model B Rev 1.5)
func getCPUModel() string {
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return ""
	}
	defer f.Close()

	model := getCPUModelFrom(f)
	if model != "" {
		return model
	}

	return getCPUModelFromLscpu()
}

// getCPUModelFrom parses an io.Reader (/proc/cpuinfo) for the
// "model name" field. Returns an empty string if the field is not found.
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

// getCPUModelFromLscpu shells out to lscpu to retrieve the CPU model name.
// On ARM SBCs, it appends the board identity from /proc/device-tree/model or
// /proc/cpuinfo's "Model" field to provide additional context.
// Returns an empty string if lscpu is unavailable or produces no output.
func getCPUModelFromLscpu() string {
	out, err := exec.Command("lscpu").Output()
	if err != nil {
		return ""
	}

	var cpuModel string
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Model name") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				cpuModel = strings.TrimSpace(parts[1])
			}
			break
		}
	}

	if cpuModel == "" {
		return ""
	}

	board := getBoardModel()
	if board != "" {
		return fmt.Sprintf("%s (%s)", cpuModel, board)
	}

	return cpuModel
}

// getBoardModel returns the hardware board name, useful for identifying
// single-board computers like Raspberry Pis. It first reads the device-tree
// model node (/proc/device-tree/model). If unavailable, it falls back to the
// "Model" field in /proc/cpuinfo. Reutrns an empty string on non-ARM systems
// or if no board info is found.
func getBoardModel() string {
	data, err := os.ReadFile("/proc/device-tree/model")
	if err == nil {
		return strings.TrimRight(string(data), "\x00")
	}

	// Fall back to /proc/cpuinfo Model field
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Model") && !strings.HasPrefix(line, "Model name") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}

	return ""
}

func getRAMTotal() uint64 {
	return memory.Total()
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
