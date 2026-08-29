package memory

import (
	"sync"
	"time"

	"github.com/nhdewitt/spectra/internal/util"
)

var (
	swapMu       sync.Mutex
	lastSwapRaw  swapRaw
	lastSwapTime time.Time
	haveSwapPrev bool
)

// swapRaw holds the cumulative page counters as read from the OS.
type swapRaw struct {
	In  uint64
	Out uint64
}

// swapRates converts the current cumulative counters into pages/second.
//
// Returns nil on the first sample of a process. Nil propagates to a SQL
// NULL, and every anomaly detector skips nulls, so an unsupported platform
// or a first sample simply reports nothing rather than zero.
func swapRates(curr swapRaw, now time.Time) (*float64, *float64) {
	swapMu.Lock()
	defer swapMu.Unlock()

	prev, prevTime, ok := lastSwapRaw, lastSwapTime, haveSwapPrev

	lastSwapRaw = curr
	lastSwapTime = now
	haveSwapPrev = true

	if !ok {
		return nil, nil
	}

	elapsed := now.Sub(prevTime).Seconds()
	if elapsed <= 0 {
		return nil, nil
	}

	in := float64(util.Delta(curr.In, prev.In)) / elapsed
	out := float64(util.Delta(curr.Out, prev.Out)) / elapsed

	return &in, &out
}
