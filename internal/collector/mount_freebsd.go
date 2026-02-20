//go:build freebsd

package collector

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// Refresh populates the FreeBSD cache with current mount points using getmntinfo.
func (c *DriveCache) Refresh() error {
	c.Lock()
	defer c.Unlock()

	// Get mount counts
	n, err := unix.Getfsstat(nil, unix.MNT_NOWAIT)
	if err != nil {
		return fmt.Errorf("getfsstat count failed: %w", err)
	}

	buf := make([]unix.Statfs_t, n)

	// Get data
	n, err = unix.Getfsstat(buf, unix.MNT_NOWAIT)
	if err != nil {
		return fmt.Errorf("getfsstat data failed: %w", err)
	}

	newMap := make(map[string]MountInfo)
	for i := range n {
		stat := buf[i]

		// Slice fixed-size arrays and convert to []byte
		// stat.Mnttoname and stat.Mntfromname are [1024]byte
		// stat.Fstypename is [16]byte
		mntPoint := unix.ByteSliceToString(stat.Mntonname[:])
		device := unix.ByteSliceToString(stat.Mntfromname[:])
		fstype := unix.ByteSliceToString(stat.Fstypename[:])

		if _, ignored := ignoredFilesystems[fstype]; ignored {
			continue
		}

		newMap[mntPoint] = MountInfo{
			Device:     device,
			Mountpoint: mntPoint,
			FSType:     fstype,
		}
	}

	c.Lock()
	c.DeviceToMountpoint = newMap
	c.Unlock()

	return nil
}
