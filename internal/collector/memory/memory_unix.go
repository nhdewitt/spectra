//go:build linux || freebsd

package memory

import (
	"context"

	"github.com/nhdewitt/spectra/internal/protocol"
	"github.com/nhdewitt/spectra/internal/util"
)

type memRaw struct {
	Total     uint64
	Available uint64
	SwapTotal uint64
	SwapFree  uint64
}

func Collect(ctx context.Context) ([]protocol.Metric, error) {
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
		UsedPct:   util.Percent(used, raw.Total),
		SwapTotal: raw.SwapTotal,
		SwapUsed:  swapUsed,
		SwapPct:   util.Percent(swapUsed, raw.SwapTotal),
	}}, nil
}
