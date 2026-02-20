//go:build linux || freebsd

package collector

import (
	"context"

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
