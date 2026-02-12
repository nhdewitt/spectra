//go:build !windows

package collector

import "sync/atomic"

// cachedMemTotal stores MemTotal in bytes, updated by CollectMemory.
// Other collectors read from this to avoid redunadant /proc/meminfo opens.
var cachedMemTotal atomic.Uint64

// MemTotal returns the cached MemTotal value in bytes
// or 0 if CollectMemory has not yet run.
func MemTotal() uint64 {
	return cachedMemTotal.Load()
}
