package diagnostics

import (
	"container/heap"
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

// RunDiskUsageTop scans root and returns the top N immediate subdirectories
// + the top N files anywhere under root.
func RunDiskUsageTop(ctx context.Context, root string, topDirsN, topFilesN int) (*protocol.DiskUsageTopReport, error) {
	start := time.Now()

	filesHeap := make(topNHeap, 0, topFilesN)
	dirsHeap := make(topNHeap, 0, topDirsN)
	heap.Init(&filesHeap)
	heap.Init(&dirsHeap)

	var scannedFiles, scannedDirs, errorCount uint64

	// Recursive walk function
	var walk func(string) (size, count uint64, err error)

	walk = func(path string) (uint64, uint64, error) {
		select {
		case <-ctx.Done():
			return 0, 0, ctx.Err()
		default:
		}

		entries, err := os.ReadDir(path)
		if err != nil {
			errorCount++
			return 0, 0, nil // Skip permission errors
		}

		scannedDirs++

		var dirSize, dirFileCount uint64

		for _, entry := range entries {
			info, err := entry.Info()
			if err != nil {
				continue
			}

			// Skip symlinks (avoid double-counts)
			if info.Mode()&os.ModeSymlink != 0 {
				continue
			}

			fullPath := filepath.Join(path, entry.Name())

			if entry.IsDir() {
				s, c, _ := walk(fullPath)
				dirSize += s
				dirFileCount += c
			} else {
				// File
				size := uint64(info.Size())
				dirSize += size
				dirFileCount++
				scannedFiles++

				pushTopN(&filesHeap, topFilesN, protocol.TopEntry{
					Path: fullPath,
					Size: size,
				})
			}
		}

		if dirSize > 0 {
			pushTopN(&dirsHeap, topDirsN, protocol.TopEntry{
				Path:  path,
				Size:  dirSize,
				Count: dirFileCount,
			})
		}

		return dirSize, dirFileCount, nil
	}

	_, _, err := walk(root)
	if err != nil {
		return nil, err
	}

	report := &protocol.DiskUsageTopReport{
		Root:         root,
		ScannedFiles: scannedFiles,
		ScannedDirs:  scannedDirs,
		ErrorCount:   errorCount,
		DurationMs:   time.Since(start).Milliseconds(),
		ScannedAt:    time.Now(),
		TopFiles:     popAllSortedDesc(&filesHeap),
		TopDirs:      popAllSortedDesc(&dirsHeap),
	}

	return report, nil
}
