//go:build darwin

package processes

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/nhdewitt/spectra/internal/collector/memory"
	"github.com/nhdewitt/spectra/internal/protocol"
)

var errBadLine = errors.New("unparseable ps line")

// On Darwin, CPUPercent comes directly from ps(1) as a
// decaying average so no delta calculation is needed.
type processRaw struct {
	PID        int
	Name       string
	State      string
	RSSBytes   uint64
	CPUPercent float64
	NumThreads uint32
}

// Collect gathers the process list on Darwin.
// This uses ps(1)'s %cpu and matches what Activity Monitor
// displays.
func Collect(ctx context.Context) ([]protocol.Metric, error) {
	procs, totalMem, err := collectRaw(ctx)
	if err != nil {
		return nil, err
	}

	results := make([]protocol.ProcessMetric, 0, len(procs))
	for _, p := range procs {
		memPercent := 0.0
		if totalMem > 0 {
			memPercent = (float64(p.RSSBytes) / float64(totalMem)) * 100.0
		}

		results = append(results, protocol.ProcessMetric{
			Pid:          p.PID,
			Name:         p.Name,
			Status:       normalizeProcState(p.State, p.CPUPercent),
			MemRSS:       p.RSSBytes,
			MemPercent:   memPercent,
			CPUPercent:   p.CPUPercent,
			ThreadsTotal: p.NumThreads,
		})
	}

	return []protocol.Metric{protocol.ProcessListMetric{
		Processes: results,
	}}, nil
}

func getRAMTotal() uint64 {
	return memory.Total()
}

// collectsRaw shells out to ps to gather process info.
func collectRaw(ctx context.Context) ([]processRaw, int64, error) {
	totalMem := getRAMTotal()

	// Add timeout to catch hangs
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// -A: all processes
	// -o: custom output with empty headers
	//
	// Darwin ps doesn't have a threads column, so we omit thread count.
	out, err := exec.CommandContext(
		ctx, "ps", "-A", "-o", "pid=,ppid=,state=,rss=,pcpu=,comm=",
	).Output()
	if err != nil {
		return nil, 0, fmt.Errorf("ps: %w", err)
	}

	procs, err := parsePsOutput(out)
	if err != nil {
		return nil, 0, err
	}

	return procs, int64(totalMem), nil
}

func parsePsOutput(data []byte) ([]processRaw, error) {
	var procs []processRaw
	scanner := bufio.NewScanner(bytes.NewReader(data))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		p, err := parsePsLine(line)
		if err != nil {
			// skip unparseable lines
			continue
		}

		procs = append(procs, p)
	}

	return procs, scanner.Err()
}

// parsePsLine parses a single line of ps output.
//
// PID PPID STATE RSS %CPU COMM
//
// Fields are whitespace-separated; COMM may contain spaces
func parsePsLine(line string) (processRaw, error) {
	fields := strings.Fields(line)
	if len(fields) < 6 {
		return processRaw{}, errBadLine
	}

	pid, err := strconv.Atoi(fields[0])
	if err != nil {
		return processRaw{}, err
	}

	rssKB, err := strconv.ParseUint(fields[3], 10, 64)
	if err != nil {
		return processRaw{}, err
	}

	cpuPct, err := strconv.ParseFloat(fields[4], 64)
	if err != nil {
		return processRaw{}, err
	}

	// rejoin any spaces in COMM
	name := strings.Join(fields[5:], " ")

	return processRaw{
		PID:        pid,
		Name:       name,
		State:      fields[2],
		RSSBytes:   rssKB * 1024,
		CPUPercent: cpuPct,
		NumThreads: 0,
	}, nil
}

// normalizeProcState maps Darwin ps state codes to protocol.ProcStatus.
// Darwin ps uses the same BSD state characters as FreeBSD:
//
// R = running/runnable
// S = sleeping
// U = uninterruptible wait (like D on Linux)
// I = idle
// T = stopped
// Z = zombie
func normalizeProcState(state string, cpuPercent float64) protocol.ProcStatus {
	if state == "" {
		return protocol.ProcOther
	}
	switch state[0] {
	case 'R':
		if cpuPercent > 0 {
			return protocol.ProcRunning
		}
		return protocol.ProcRunnable
	case 'S', 'U', 'I':
		return protocol.ProcWaiting
	case 'T', 'Z':
		return protocol.ProcOther
	default:
		return protocol.ProcOther
	}
}
