//go:build !windows
// +build !windows

package collector

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/nhdewitt/spectra/internal/protocol"
)

type memRaw struct {
	Total     uint64
	Available uint64
	SwapTotal uint64
	SwapFree  uint64
}

func CollectMemory(ctx context.Context) ([]protocol.Metric, error) {
	raw, err := parseMemInfo()
	if err != nil {
		return nil, err
	}

	used := raw.Total - raw.Available
	swapUsed := raw.SwapTotal - raw.SwapFree

	return []protocol.Metric{protocol.MemoryMetric{
		Total:     raw.Total,
		Available: raw.Available,
		Used:      used,
		UsedPct:   percent(used, raw.Total),
		SwapTotal: raw.SwapTotal,
		SwapUsed:  swapUsed,
		SwapPct:   percent(swapUsed, raw.SwapTotal),
	}}, nil
}

func parseMemInfo() (memRaw, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return memRaw{}, fmt.Errorf("opening /proc/meminfo: %w", err)
	}
	defer f.Close()

	var raw memRaw

	targets := map[string]*uint64{
		"MemTotal":     &raw.Total,
		"MemAvailable": &raw.Available,
		"SwapTotal":    &raw.SwapTotal,
		"SwapFree":     &raw.SwapFree,
	}

	found := 0
	scanner := bufio.NewScanner(f)

	for scanner.Scan() && found < len(targets) {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}

		key := strings.TrimSuffix(fields[0], ":")
		target, ok := targets[key]
		if !ok {
			continue
		}

		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return memRaw{}, fmt.Errorf("parsing %s: %w", key, err)
		}

		*target = value
		found++
	}

	if err := scanner.Err(); err != nil {
		return memRaw{}, fmt.Errorf("reading /proc/meminfo: %w", err)
	}
	if found < len(targets) {
		return memRaw{}, fmt.Errorf("missing fields in /proc/meminfo: found %d of %d", found, len(targets))
	}

	return raw, nil
}
