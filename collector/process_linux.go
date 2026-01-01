//go:build !windows

package collector

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/nhdewitt/spectra/metrics"
)

// processState stores the last CPU ticks for a PID
type processState struct {
	lastTicks uint64
	lastTime  time.Time
}

// pidStatRaw holds the raw values parsed from /proc/[pid]/stat
type pidStatRaw struct {
	Name       string
	State      string
	PPID       int
	UTime      uint64
	STime      uint64
	RSSPages   uint64
	TotalTicks uint64
}

var (
	lastProcessStates = make(map[int]processState)
	clkTck            = 100.0
)

func CollectProcesses(ctx context.Context) ([]metrics.Metric, error) {
	// Get Total Memory
	memFile, err := os.Open("/proc/meminfo")
	if err != nil {
		return nil, err
	}
	defer memFile.Close()

	totalMem, err := parseMemInfoFrom(memFile)
	if err != nil {
		return nil, err
	}

	// List PIDs
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}

	var results []metrics.Metric
	currentStates := make(map[int]processState)
	now := time.Now()
	pageSize := uint64(os.Getpagesize())

	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}

		// Parse /proc/[pid]/stat
		f, err := os.Open(filepath.Join("/proc", entry.Name(), "stat"))
		if err != nil {
			continue
		}

		stat, err := parsePidStatFrom(f)
		f.Close()
		if err != nil {
			continue
		}

		memRSS := stat.RSSPages * pageSize

		memPercent := 0.0
		if totalMem > 0 {
			memPercent = (float64(memRSS) / float64(totalMem)) * 100.0
		}

		cpuPercent := 0.0
		if prevState, ok := lastProcessStates[pid]; ok {
			deltaTicks := float64(stat.TotalTicks - prevState.lastTicks)
			deltaTime := now.Sub(prevState.lastTime).Seconds()

			if deltaTime > 0 {
				cpuPercent = ((deltaTicks / clkTck) / deltaTime) * 100.0
			}
		}

		currentStates[pid] = processState{
			lastTicks: stat.TotalTicks,
			lastTime:  now,
		}

		results = append(results, metrics.ProcessMetric{
			Pid:        pid,
			Name:       stat.Name,
			Status:     stat.State,
			MemRSS:     memRSS,
			MemPercent: memPercent,
			CPUPercent: cpuPercent,
		})
	}

	lastProcessStates = currentStates
	return results, nil
}

// parseMemInfoFrom parses /proc/meminfo to find MemTotal in bytes.
func parseMemInfoFrom(r io.Reader) (uint64, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return 0, err
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, err := strconv.ParseUint(fields[1], 10, 64)
				if err != nil {
					return 0, err
				}
				return kb * 1024, nil
			}
		}
	}

	return 0, fmt.Errorf("MemTotal not found")
}

// parsePidStatFrom parses a single line from /proc/[pid]/stat
func parsePidStatFrom(r io.Reader) (*pidStatRaw, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	str := string(data)

	firstParen := strings.Index(str, "(")
	lastParen := strings.LastIndex(str, ")")
	if firstParen == -1 || lastParen == -1 || lastParen <= firstParen {
		return nil, fmt.Errorf("invalid format")
	}

	name := str[firstParen+1 : lastParen]

	rest := str[lastParen+2:]
	fields := strings.Fields(rest)
	if len(fields) < 22 {
		return nil, fmt.Errorf("insufficient fields")
	}

	parse := makeUintParser(fields, "process")

	// Indices shifted:
	// State (2) -> 0
	// PPID (3) -> 1
	// utime (13) -> 11
	// stime (14) -> 12
	// rss (23) -> 21

	ppid, _ := strconv.Atoi(fields[1])
	utime := parse(11)
	stime := parse(12)
	rss := parse(21)

	return &pidStatRaw{
		Name:       name,
		State:      fields[0],
		PPID:       ppid,
		UTime:      utime,
		STime:      stime,
		RSSPages:   rss,
		TotalTicks: utime + stime,
	}, nil
}
