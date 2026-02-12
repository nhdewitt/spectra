//go:build !windows
// +build !windows

package collector

import (
	"bufio"
	"context"
	"fmt"
	"io"
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
	cachedMemTotal.Store(raw.Total)

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

	return parseMemInfoFrom(f)
}

func parseMemInfoFrom(r io.Reader) (memRaw, error) {
	var raw memRaw

	targets := map[string]*uint64{
		"MemTotal":     &raw.Total,
		"MemAvailable": &raw.Available,
		"SwapTotal":    &raw.SwapTotal,
		"SwapFree":     &raw.SwapFree,
	}

	scanner := bufio.NewScanner(r)

	for scanner.Scan() && len(targets) > 0 {
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

		*target = value * 1024
		// Remove the key to prevent duplicates from changing the value
		delete(targets, key)
	}

	if err := scanner.Err(); err != nil {
		return memRaw{}, fmt.Errorf("reading /proc/meminfo: %w", err)
	}
	if len(targets) > 0 {
		missing := make([]string, 0, len(targets))
		for k := range targets {
			missing = append(missing, k)
		}
		return memRaw{}, fmt.Errorf("missing fields in /proc/meminfo: %v", missing)
	}

	return raw, nil
}
