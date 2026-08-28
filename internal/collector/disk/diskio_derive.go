package disk

// Derivations shared by every platform's disk IO collector.
//
// These exist in one place because the inputs differ per platform, but the
// arithmetic must not. Linux reports service time in ms, Windows in 100ns
// ticks, Darwin in ns. Each collector converts to ms before calling these,
// and the definition of "latency" stays identical across the fleet.

// AwaitMs is the average service time per operation, in ms.
//
// This is what iostat calls r_await/w_await. Both arguments must be raw
// deltas over the same interval. Passing a rate-converted ops count and a
// raw time delta yields a figure wrong by the length of the interval,
// which is the bug this function replaces.
//
// Returns 0 when no operations completed.
func AwaitMs(busyMs, ops uint64) float64 {
	if ops == 0 {
		return 0
	}
	return float64(busyMs) / float64(ops)
}

// BusyPct is the share of the interval the device spent servicing IO,
// which is what iostat calls %util.
//
// Can legitimately exceed 100 on a device that services requests in
// parallel because the underlying counter sums the service time of every
// concurrent request.
func BusyPct(busyMs uint64, elapsed float64) float64 {
	if elapsed <= 0 {
		return 0
	}
	return float64(busyMs) / (elapsed * 1000) * 100
}
