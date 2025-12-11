//go:build windows
// +build windows

package collector

import (
	"context"
	"fmt"
	"log"

	"github.com/nhdewitt/raspimon/metrics"
	"github.com/shirou/gopsutil/v3/mem"
)

func CollectMemory(ctx context.Context) ([]metrics.Metric, error) {
	v, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get virtual memory stats: %w", err)
	}

	s, err := mem.SwapMemoryWithContext(ctx)
	if err != nil {
		log.Printf("Failed to get swap memory stats: %v", err)
		s = &mem.SwapMemoryStat{}
	}

	return []metrics.Metric{metrics.MemoryMetric{
		Total:     v.Total,
		Used:      v.Used,
		Available: v.Available,
		UsedPct:   percent(v.Used, v.Total),
		SwapTotal: s.Total,
		SwapUsed:  s.Used,
		SwapPct:   percent(s.Used, s.Total),
	}}, nil
}
