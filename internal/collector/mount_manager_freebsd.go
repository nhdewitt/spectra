//go:build freebsd

package collector

import "golang.org/x/sys/unix"

func parseMounts() ([]MountInfo, error) {
	// Get count of mounted filesystems.
	n, err := unix.Getfsstat(nil, unix.MNT_NOWAIT)
	if err != nil {
		return nil, err
	}

	buf := make([]unix.Statfs_t, n)
	n, err = unix.Getfsstat(buf, unix.MNT_NOWAIT)
	if err != nil {
		return nil, err
	}

	var mounts []MountInfo
	for _, fs := range buf[:n] {
		m := MountInfo{
			Device:     unix.ByteSliceToString(fs.Mntfromname[:]),
			Mountpoint: unix.ByteSliceToString(fs.Mntonname[:]),
			FSType:     unix.ByteSliceToString(fs.Fstypename[:]),
		}

		if shouldIgnore(m) {
			continue
		}

		mounts = append(mounts, m)
	}

	return mounts, nil
}
