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

	"github.com/nhdewitt/spectra/internal/protocol"
	"github.com/tklauser/go-sysconf"
)

func init() {
	if sc, err := sysconf.Sysconf(sysconf.SC_CLK_TCK); err == nil && sc > 0 {
		clkTck = float64(sc)
	}
}

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
	NumThreads uint32
}

var (
	lastProcessStates = make(map[int]processState)
	clkTck            = 100.0
)

func CollectProcesses(ctx context.Context) ([]protocol.Metric, error) {
	// Get Total Memory
	memFile, err := os.Open("/proc/meminfo")
	if err != nil {
		return nil, err
	}
	defer memFile.Close()

	totalMem, err := parseProcessMemInfoFrom(memFile)
	if err != nil {
		return nil, err
	}

	// List PIDs
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}

	var results []protocol.ProcessMetric
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

		results = append(results, protocol.ProcessMetric{
			Pid:          pid,
			Name:         stat.Name,
			Status:       normalizeLinuxProcState(stat.State, cpuPercent),
			MemRSS:       memRSS,
			MemPercent:   memPercent,
			CPUPercent:   cpuPercent,
			ThreadsTotal: stat.NumThreads,
		})
	}

	lastProcessStates = currentStates
	return []protocol.Metric{
		protocol.ProcessListMetric{Processes: results},
	}, nil
}

// parseMemInfoFrom parses /proc/meminfo to find MemTotal in bytes.
func parseProcessMemInfoFrom(r io.Reader) (uint64, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return 0, err
	}

	lines := strings.SplitSeq(string(data), "\n")
	for line := range lines {
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
	// PPID (4) -> 1
	// utime (14) -> 11
	// stime (15) -> 12
	// num_threads (20) -> 17
	// rss (24) -> 21

	ppid, _ := strconv.Atoi(fields[1])
	utime := parse(11)
	stime := parse(12)
	numThreads := parse(17)
	rss := parse(21)

	return &pidStatRaw{
		Name:       name,
		State:      fields[0],
		PPID:       ppid,
		UTime:      utime,
		STime:      stime,
		RSSPages:   rss,
		TotalTicks: utime + stime,
		NumThreads: uint32(numThreads),
	}, nil
}

func normalizeLinuxProcState(state string, cpuPercent float64) protocol.ProcStatus {
	if state == "" {
		return protocol.ProcOther
	}

	switch state[0] {
	case 'R':
		// "running" or "runnable"
		// If it used CPU in the sample -> running; otherwise -> runnable
		if cpuPercent > 0 {
			return protocol.ProcRunning
		}
		return protocol.ProcRunnable

	case 'S', 'D', 'I', 'W':
		return protocol.ProcWaiting

	case 'T', 't', 'Z', 'X':
		return protocol.ProcOther

	default:
		return protocol.ProcOther
	}
}
