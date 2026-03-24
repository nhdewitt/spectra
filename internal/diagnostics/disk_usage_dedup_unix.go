//go:build linux || freebsd || darwin

package diagnostics

import (
	"os"
	"syscall"
)

func fileKey(info os.FileInfo) ([2]uint64, bool) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return [2]uint64{}, false
	}
	return [2]uint64{uint64(stat.Dev), stat.Ino}, true
}
