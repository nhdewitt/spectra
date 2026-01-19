//go:build windows

package collector

import (
	"context"
	"strings"
	"testing"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func TestCollectDisk_Integration(t *testing.T) {
	metric, err := CollectDisk(context.Background())
	if err != nil {
		t.Fatalf("CollectDisk failed: %v", err)
	}

	if len(metric) == 0 {
		t.Log("No fixed drives found.")
		return
	}

	foundCDrive := false
	for _, m := range metric {
		dm := m.(protocol.DiskMetric)

		t.Logf("Found Drive: %s (%s) %s", dm.Mountpoint, dm.Filesystem, dm.Device)

		if dm.Total == 0 {
			t.Errorf("Drive %s has 0 total bytes", dm.Mountpoint)
		}

		if strings.EqualFold(dm.Mountpoint, `C:\`) {
			foundCDrive = true
		}
	}

	if !foundCDrive {
		t.Log("Warning: C:\\ drive was not found.")
	}
}

func BenchmarkCollectDisk(b *testing.B) {
	ctx := context.Background()
	b.ReportAllocs()

	for b.Loop() {
		_, _ = CollectDisk(ctx)
	}
}
