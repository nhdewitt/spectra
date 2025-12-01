package collector

import (
	"context"
	"testing"
)

func setupMountCache(b *testing.B) *MountMap {
	cache := &MountMap{
		DeviceToMountpoint: make(map[string]MountInfo),
	}

	mounts, err := parseMounts()
	if err != nil {
		b.Fatalf("failed to parse mounts for benchmark setup: %v", err)
	}

	newMap := createDeviceToMountpointMap(mounts)
	cache.DeviceToMountpoint = newMap

	return cache
}

func BenchmarkCollectDisk(b *testing.B) {
	ctx := context.Background()
	mountCache := setupMountCache(b)

	diskCollector := MakeDiskCollector(mountCache)
	b.ResetTimer()

	for b.Loop() {
		diskCollector(ctx)
	}
}
