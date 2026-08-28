//go:build linux || freebsd || darwin

package memory

import (
	"testing"
	"time"
)

// resetSwapState clears the cached sample between cases.
//
// swapRates keeps package-level state so it can turn cumulative counters into
// a rate, which means tests are order-dependent unless each one starts clean.
func resetSwapState() {
	swapMu.Lock()
	defer swapMu.Unlock()
	lastSwapRaw = swapRaw{}
	lastSwapTime = time.Time{}
	haveSwapPrev = false
}

func TestSwapRates_FirstSampleReportsNothing(t *testing.T) {
	resetSwapState()

	// A rate needs two readings. Returning 0 here would be indistinguishable
	// from a genuinely idle host, so the first sample must report nil.
	in, out := swapRates(swapRaw{In: 100, Out: 200}, time.Now())
	if in != nil || out != nil {
		t.Errorf("first sample: want (nil, nil), got (%v, %v)", in, out)
	}
}

func TestSwapRates_ComputesPagesPerSecond(t *testing.T) {
	resetSwapState()
	start := time.Now()

	swapRates(swapRaw{In: 1000, Out: 2000}, start)
	in, out := swapRates(swapRaw{In: 1600, Out: 2300}, start.Add(10*time.Second))

	if in == nil || out == nil {
		t.Fatalf("second sample: want rates, got (%v, %v)", in, out)
	}
	if *in != 60 {
		t.Errorf("swap in: want 60 pages/s, got %v", *in)
	}
	if *out != 30 {
		t.Errorf("swap out: want 30 pages/s, got %v", *out)
	}
}

func TestSwapRates_ClampsCounterReset(t *testing.T) {
	resetSwapState()
	start := time.Now()

	swapRates(swapRaw{In: 5000, Out: 5000}, start)

	// /proc/vmstat zeroes on reboot. Bare subtraction would underflow to
	// ~1.8e19 pages/s and swamp every real value on the chart.
	in, out := swapRates(swapRaw{In: 10, Out: 20}, start.Add(10*time.Second))

	if in == nil || out == nil {
		t.Fatalf("after reset: want rates, got (%v, %v)", in, out)
	}
	if *in != 0 || *out != 0 {
		t.Errorf("after reset: want (0, 0), got (%v, %v)", *in, *out)
	}
}

func TestSwapRates_ZeroElapsedReportsNothing(t *testing.T) {
	resetSwapState()
	now := time.Now()

	swapRates(swapRaw{In: 100, Out: 100}, now)

	// Two samples at the same instant would divide by zero.
	in, out := swapRates(swapRaw{In: 500, Out: 500}, now)
	if in != nil || out != nil {
		t.Errorf("zero elapsed: want (nil, nil), got (%v, %v)", in, out)
	}
}

func TestSwapRates_IdleHostReportsZeroNotNil(t *testing.T) {
	resetSwapState()
	start := time.Now()

	swapRates(swapRaw{In: 42, Out: 42}, start)
	in, out := swapRates(swapRaw{In: 42, Out: 42}, start.Add(30*time.Second))

	// Zero is a real measurement: nothing paged. It must NOT collapse to nil,
	// which means "cannot measure" and would hide a healthy host's chart.
	if in == nil || out == nil {
		t.Fatalf("idle host: want (0, 0), got (%v, %v)", in, out)
	}
	if *in != 0 || *out != 0 {
		t.Errorf("idle host: want (0, 0), got (%v, %v)", *in, *out)
	}
}
