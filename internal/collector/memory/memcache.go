//go:build linux || darwin

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

func Total() uint64 {
	if v := cachedMemTotal.Load(); v > 0 {
		return v
	}

	raw, err := parseMemInfo()
	if err != nil {
		return 0
	}

	cachedMemTotal.Store(raw.Total)
	return raw.Total
}
