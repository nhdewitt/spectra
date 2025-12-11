package collector

import (
	"context"

	"github.com/nhdewitt/raspimon/metrics"
)

// MountMap filters Linux drives for DiskIO collection
// type MountMap struct {
// 	sync.RWMutex
// 	DeviceToMountpoint map[string]MountInfo
// }

// DriveMap filters Windows drives for DiskIO collection
// type DriveMap struct {
// 	sync.RWMutex
// 	AllowedDrives map[uint32]Win32_DiskDrive
// }

// type MountInfo struct {
// 	Device     string
// 	Mountpoint string
// 	FSType     string
// }

// CollectFunc is any function that produces a metric
type CollectFunc func(context.Context) ([]metrics.Metric, error)

// type Win32_DiskDrive struct {
// 	DeviceID      string
// 	InterfaceType string
// 	MediaType     string
// 	Model         string
// 	Index         uint32
// }
