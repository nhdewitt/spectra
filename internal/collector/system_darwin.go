//go:build darwin

package collector

import (
	"bytes"
	"context"
	"encoding/binary"
	"os/exec"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
	"golang.org/x/sys/unix"
)

// CollectSystem gathers system-level metrics on Darwin.
//
// Uptime+Boottime: sysctl kern.boottime
// Process count: ps -Axo pid= (line count)
// User count: shared with linux/freebsd who parse
func CollectSystem(ctx context.Context) ([]protocol.Metric, error) {
	bootTime, uptime, err := getBootTimeAndUptime()
	if err != nil {
		return nil, err
	}

	procCount, err := countProcesses(ctx)
	if err != nil {
		return nil, err
	}

	out, _ := exec.CommandContext(ctx, "who").Output()
	users := parseWhoFrom(bytes.NewReader(out))

	return []protocol.Metric{protocol.SystemMetric{
		Uptime:    uptime,
		BootTime:  bootTime,
		Processes: procCount,
		Users:     users,
	}}, nil
}

// getBootTimeAndUptime reads kern.boottime via sysctl.
func getBootTimeAndUptime() (bootTime, uptime uint64, err error) {
	raw, err := unix.SysctlRaw("kern.boottime")
	if err != nil {
		return 0, 0, err
	}

	// kern.boottime returns a struct timeval { time_t tv_sec; suseconds_t tv_usec; }
	// On darwin, tv_sec is int64, can ignore tv_usec for calculation.
	var sec int64
	if err := binary.Read(bytes.NewReader(raw), binary.LittleEndian, &sec); err != nil {
		return 0, 0, err
	}

	bootTime = uint64(sec)
	uptime = uint64(time.Now().Unix()) - bootTime

	return bootTime, uptime, nil
}

// countProcesses runs ps -Axo pid= and counts output lines to get the number of
// running processes.
func countProcesses(ctx context.Context) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "ps", "-Axo", "pid=").Output()
	if err != nil {
		return 0, err
	}

	return bytes.Count(out, []byte("\n")), nil
}
