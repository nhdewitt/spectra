//go:build !windows

package collector

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/nhdewitt/spectra/metrics"
)

func CollectSystem(ctx context.Context) ([]metrics.Metric, error) {
	// Uptime & Boottime - /proc/uptime
	f, err := os.Open("/proc/uptime")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	uptime, bootTime, err := parseProcUptimeFrom(f)
	if err != nil {
		return nil, err
	}

	// Process Count
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}

	var dirNames []string
	for _, e := range entries {
		dirNames = append(dirNames, e.Name())
	}
	processCount := countProcs(dirNames)

	// User Count
	out, _ := exec.CommandContext(ctx, "who").Output()
	users := parseWhoFrom(bytes.NewReader(out))

	return []metrics.Metric{
		metrics.SystemMetric{
			Uptime:    uptime,
			BootTime:  bootTime,
			Processes: processCount,
			Users:     users,
		},
	}, nil
}

// parseProcUptimeFrom parses /proc/uptime.
func parseProcUptimeFrom(r io.Reader) (uptime, bootTime uint64, err error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return 0, 0, err
	}

	parts := strings.Fields(string(data))
	if len(parts) == 0 {
		return 0, 0, io.ErrUnexpectedEOF
	}

	uptimeInSeconds, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0, 0, err
	}

	uptime = uint64(uptimeInSeconds)
	bootTime = uint64(time.Now().Unix()) - uptime

	return uptime, bootTime, nil
}

// countProcs returns how many strings in the slice are numeric.
func countProcs(entries []string) int {
	count := 0
	for _, name := range entries {
		if _, err := strconv.Atoi(name); err == nil {
			count++
		}
	}

	return count
}

// parseWhoFrom counts lines in the output of the `who` command.
func parseWhoFrom(r io.Reader) int {
	data, err := io.ReadAll(r)
	if err != nil {
		return 0
	}

	s := strings.TrimSpace(string(data))
	if s == "" {
		return 0
	}

	return len(strings.Split(s, "\n"))
}
