//go:build windows
// +build windows

package collector

import (
	"context"

	"github.com/nhdewitt/raspimon/metrics"
)

func CollectDisk_Windows(ctx context.Context) ([]metrics.Metric, error) {
	return nil, nil
}
