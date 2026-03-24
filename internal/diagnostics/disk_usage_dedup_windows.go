//go:build windows

package diagnostics

import "os"

func fileKey(_ os.FileInfo) ([2]uint64, bool) {
	return [2]uint64{}, false
}
