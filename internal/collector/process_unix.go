//go:build linux || freebsd

package collector

import (
	"context"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

// processState stores the last CPU ticks for a PID.
type processState struct {
	lastTicks uint64
	lastTime  time.Time
}

// processRaw is the intermediate representation returned
// by collectProcessRaw on each platform.
type processRaw struct {
	PID        int
	Name       string
	State      string
	RSSBytes   uint64
	TotalTicks uint64 // cumulative CPU ticks (utime + stime)
	NumThreads uint32
}

var lastProcessStates = make(map[int]processState)

func CollectProcesses(ctx context.Context) ([]protocol.Metric, error) {
	procs, totalMem, err := collectProcessRaw()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	currentStates := make(map[int]processState, len(procs))
	results := make([]protocol.ProcessMetric, 0, len(procs))

	for _, p := range procs {
		memPercent := 0.0
		if totalMem > 0 {
			memPercent = (float64(p.RSSBytes) / float64(totalMem)) * 100.0
		}

		cpuPercent := 0.0
		if prev, ok := lastProcessStates[p.PID]; ok {
			deltaTicks := float64(p.TotalTicks - prev.lastTicks)
			deltaTime := now.Sub(prev.lastTime).Seconds()
			if deltaTime > 0 {
				cpuPercent = ((deltaTicks / clkTck) / deltaTime) * 100.0
			}
		}

		currentStates[p.PID] = processState{
			lastTicks: p.TotalTicks,
			lastTime:  now,
		}

		results = append(results, protocol.ProcessMetric{
			Pid:          p.PID,
			Name:         p.Name,
			Status:       normalizeProcState(p.State, cpuPercent),
			MemRSS:       p.RSSBytes,
			MemPercent:   memPercent,
			CPUPercent:   cpuPercent,
			ThreadsTotal: p.NumThreads,
		})
	}

	lastProcessStates = currentStates

	return []protocol.Metric{
		protocol.ProcessListMetric{Processes: results},
	}, nil
}

func normalizeProcState(state string, cpuPercent float64) protocol.ProcStatus {
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
