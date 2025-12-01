package collector

import (
	"context"
	"testing"
)

func BenchmarkCollectDiskIO(b *testing.B) {
	ctx := context.Background()
	mountCache := setupMountCache(b)

	diskIOCollector := MakeDiskIOCollector(mountCache)
	b.ResetTimer()

	for b.Loop() {
		diskIOCollector(ctx)
	}
}
