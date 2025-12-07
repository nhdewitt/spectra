//go:build windows
// +build windows

package collector

import (
	"context"
	"time"
)

func RunMountManager_Windows(ctx context.Context, cache *MountMap, interval time.Duration) {
	<-ctx.Done()
}

func MakeDiskCollector_Windows(cache *MountMap) CollectFunc {
	return CollectDisk_Windows
}

func MakeDiskIOCollector_Windows(cache *MountMap) CollectFunc {
	return CollectDiskIO_Windows
}
