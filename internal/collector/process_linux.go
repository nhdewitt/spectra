//go:build linux

package collector

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tklauser/go-sysconf"
)

func init() {
	if sc, err := sysconf.Sysconf(sysconf.SC_CLK_TCK); err == nil && sc > 0 {
		clkTck = float64(sc)
	}
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
	clkTck = 100.0
)

// collectProcessRaw reads /proc to enumerate all processes.
// Returns process list and total system memory (in bytes).
func collectProcessRaw() ([]processRaw, uint64, error) {
	// Get total memory
	memFile, err := os.Open("/proc/meminfo")
	if err != nil {
		return nil, 0, err
	}
	defer memFile.Close()

	totalMem, err := parseProcessMemInfoFrom(memFile)
	if err != nil {
		return nil, 0, err
	}

	// List PIDs
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, 0, err
	}

	pageSize := uint64(os.Getpagesize())
	var procs []processRaw

	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}

		f, err := os.Open(filepath.Join("/proc", entry.Name(), "stat"))
		if err != nil {
			continue
		}

		stat, err := parsePidStatFrom(f)
		f.Close()
		if err != nil {
			continue
		}

		procs = append(procs, processRaw{
			PID:        pid,
			Name:       stat.Name,
			State:      stat.State,
			RSSBytes:   stat.RSSPages * pageSize,
			TotalTicks: stat.TotalTicks,
			NumThreads: stat.NumThreads,
		})
	}

	return procs, totalMem, nil
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
