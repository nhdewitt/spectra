package memory

import "sync/atomic"

// cachedMemTotal stores MemTotal in bytes, updated by CollectMemory.
// Other collectors read from this to avoid redunadant /proc/meminfo opens.
var cachedMemTotal atomic.Uint64

// MemTotal returns the cached MemTotal value in bytes
// or 0 if CollectMemory has not yet run.
func MemTotal() uint64 {
	return cachedMemTotal.Load()
}

// Total returns total physical memory in bytes, reading it from the platform
// on first call and caching it thereafter.
//
// Returns 0 when the platform read fails. Callers must treat 0 as "unknown"
// and fall back to a fixed default rather than deriving a limit from it.
func Total() uint64 {
	if v := cachedMemTotal.Load(); v > 0 {
		return v
	}

	total, err := totalPhysical()
	if err != nil || total == 0 {
		return 0
	}

	cachedMemTotal.Store(total)
	return total
}
