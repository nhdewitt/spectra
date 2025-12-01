package collector

import (
	"context"
	"testing"
)

func BenchmarkCollectCPU(b *testing.B) {
	ctx := context.Background()

	for b.Loop() {
		CollectCPU(ctx)
	}
}
