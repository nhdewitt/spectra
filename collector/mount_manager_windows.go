//go:build windows
// +build windows

package collector

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/nhdewitt/raspimon/metrics"
	"github.com/yusufpapurcu/wmi"
)

var ignoredInterfaceTypes = map[string]struct{}{
	"USB":  {},
	"1394": {},
}

func queryPhysicalDrives() ([]Win32_DiskDrive, error) {
	var drives []Win32_DiskDrive
	query := "SELECT DeviceID, InterfaceType, MediaType, Model, Index FROM Win32_DiskDrive"
	err := wmi.Query(query, &drives)
	if err != nil {
		return nil, fmt.Errorf("WMI query failed: %w", err)
	}
	return drives, nil
}

func filterDrives(drives []Win32_DiskDrive) []Win32_DiskDrive {
	allowed := make([]Win32_DiskDrive, 0, len(drives))
	for _, d := range drives {
		if _, ok := ignoredInterfaceTypes[d.InterfaceType]; ok {
			continue
		}

		if strings.Contains(strings.ToLower(d.Model), "virtual") {
			continue
		}
		allowed = append(allowed, d)
	}
	return allowed
}

func createDriveIndexMap(drives []Win32_DiskDrive) map[uint32]Win32_DiskDrive {
	m := make(map[uint32]Win32_DiskDrive)
	for _, d := range drives {
		m[d.Index] = d
	}
	return m
}

func RunMountManager(ctx context.Context, cache *DriveCache, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	updateDriveCache(cache)

	for {
		select {
		case <-ticker.C:
			updateDriveCache(cache)
		case <-ctx.Done():
			log.Println("Mount Manager stopped.")
			return
		}
	}
}

func updateDriveCache(cache *DriveCache) {
	drives, err := queryPhysicalDrives()
	if err != nil {
		log.Printf("WARNING: Error querying drives: %v", err)
		return
	}

	allowed := filterDrives(drives)
	newMap := createDriveIndexMap(allowed)

	letterMap, err := buildDriveLetterMap()
	if err != nil {
		log.Printf("WARNING: Error building drive letter map: %v", err)
	}

	cache.RWMutex.Lock()
	cache.AllowedDrives = newMap
	cache.DriveLetterMap = letterMap
	cache.RWMutex.Unlock()
}

// buildDriveLetterMap creates a mapping from physical drive index to drive letters
func buildDriveLetterMap() (map[uint32][]string, error) {
	// Physical drive to partition mappings
	var driveToPartition []Win32_DiskDriveToDiskPartition
	err := wmi.Query("SELECT Antecedent, Dependent FROM Win32_DiskDriveToDiskPartition", &driveToPartition)
	if err != nil {
		return nil, fmt.Errorf("failed to query drive to disk partition: %w", err)
	}

	// Partition to logical disk mappings
	var partitionToLogical []Win32_LogicalDiskToPartition
	err = wmi.Query("SELECT Antecedent, Dependent FROM Win32_LogicalDiskToPartition", &partitionToLogical)
	if err != nil {
		return nil, fmt.Errorf("failed to query partition to logical: %w", err)
	}

	// Partition -> drive letter map
	partitionToLetter := make(map[string]string)
	for _, p := range partitionToLogical {
		letter := extractDriveLetter(p.Dependent)
		partition := extractPartitionName(p.Antecedent)
		if letter != "" && partition != "" {
			partitionToLetter[partition] = letter
		}
	}

	// Physical drive -> drive letter map
	result := make(map[uint32][]string)
	for _, d := range driveToPartition {
		driveIndex := extractDriveIndex(d.Antecedent)
		partition := extractPartitionName(d.Dependent)

		if driveIndex >= 0 && partition != "" {
			if letter, ok := partitionToLetter[partition]; ok {
				result[uint32(driveIndex)] = append(result[uint32(driveIndex)], letter)
			}
		}
	}

	return result, nil
}

// extractDriveLetter extracts the drive letter from the WMI query.
func extractDriveLetter(wmiPath string) string {
	re := regexp.MustCompile(`DeviceID="([A-Z]:)"`)
	matches := re.FindStringSubmatch(wmiPath)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}

// extractPartitionName extracts the partition name from the WMI query.
func extractPartitionName(wmiPath string) string {
	re := regexp.MustCompile(`DeviceID="(Disk #\d+, Partition #\d+)"`)
	matches := re.FindStringSubmatch(wmiPath)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}

// extractDriveIndex extracts the drive index from the WMI query.
func extractDriveIndex(wmiPath string) int {
	re := regexp.MustCompile(`PHYSICALDRIVE(\d+)`)
	matches := re.FindStringSubmatch(strings.ToUpper(wmiPath))
	if len(matches) >= 2 {
		idx, err := strconv.Atoi(matches[1])
		if err == nil {
			return idx
		}
	}
	return -1
}

func MakeDiskCollector(cache *DriveCache) CollectFunc {
	return func(ctx context.Context) ([]metrics.Metric, error) {
		return CollectDisk(ctx)
	}
}

func MakeDiskIOCollector(cache *DriveCache) CollectFunc {
	return func(ctx context.Context) ([]metrics.Metric, error) {
		return CollectDiskIO(ctx, cache)
	}
}
