//go:build darwin && !cgo

package collector

import (
	"bufio"
	"bytes"
	"context"
	"os/exec"
	"strconv"
	"strings"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func CollectCPU(ctx context.Context) ([]protocol.Metric, error) {
	usage, err := cpuUsageFromTop(ctx)
	if err != nil {
		usage = 0
	}

	load1, load5, load15, err := parseLoadAvg()
	if err != nil {
		return nil, err
	}

	return []protocol.Metric{protocol.CPUMetric{
		Usage:     usage,
		LoadAvg1:  load1,
		LoadAvg5:  load5,
		LoadAvg15: load15,
		// CoreUsage: nil - unavailable without cgo
		// IOWait: 0 - not present on darwin
	}}, nil
}

// cpuUsageFromTop parses "top -l 2 -n 0 -s 1" to get CPU usage.
// Take the second sample.
func cpuUsageFromTop(ctx context.Context) (float64, error) {
	out, err := exec.CommandContext(ctx, "top", "-l", "2", "-n", "0", "-s", "1").Output()
	if err != nil {
		return 0, err
	}

	return parseCPUFromTop(out)
}

func parseCPUFromTop(out []byte) (float64, error) {
	var cpuLine string
	scanner := bufio.NewScanner(bytes.NewReader(out))

	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "CPU usage:") {
			cpuLine = scanner.Text()
		}
	}

	if cpuLine == "" {
		return 0, nil
	}

	return parseCPUUsageLine(cpuLine)
}

func parseCPUUsageLine(line string) (float64, error) {
	line = strings.TrimPrefix(line, "CPU usage: ")
	parts := strings.Split(line, ",")

	var user, sys float64
	for _, part := range parts {
		part = strings.TrimSpace(part)
		fields := strings.Fields(part)
		if len(fields) != 2 {
			continue
		}
		val, err := strconv.ParseFloat(strings.TrimSuffix(fields[0], "%"), 64)
		if err != nil {
			continue
		}

		switch fields[1] {
		case "user":
			user = val
		case "sys":
			sys = val
		}
	}

	return user + sys, nil
}
