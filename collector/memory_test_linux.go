//go:build !windows
// +build !windows

package collector

import (
	"context"
	"testing"
)

func BenchmarkCollectMemory(b *testing.B) {
	ctx := context.Background()

	for b.Loop() {
		CollectMemory(ctx)
	}
}
