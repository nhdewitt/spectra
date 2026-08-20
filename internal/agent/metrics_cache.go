package agent

import (
	"sync"

	"github.com/nhdewitt/spectra/internal/protocol"
)

const defaultMaxCacheSize = 10_000

// metricsCache holds unsent metric envelopes for retry when the server
// is unreachable. Uses a bounded buffer to prevent unbounded memory
// growth on resource-constrained devices.
type metricsCache struct {
	mu      sync.Mutex
	pending []protocol.Envelope
	maxSize int
}

func newMetricsCache(maxSize int) *metricsCache {
	if maxSize <= 0 {
		maxSize = defaultMaxCacheSize
	}
	return &metricsCache{
		pending: make([]protocol.Envelope, 0, 64),
		maxSize: maxSize,
	}
}

// Add appends failed envelopes to the cache. If the cache exceeds maxSize,
// the oldest envelopes are removed.
func (c *metricsCache) Add(batch []protocol.Envelope) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.pending = append(c.pending, batch...)

	if len(c.pending) > c.maxSize {
		c.pending = c.pending[len(c.pending)-c.maxSize:]
	}
}

// Drain returns all cached envelopes and clears the cache.
// Returns nil if the cache is empty.
func (c *metricsCache) Drain() []protocol.Envelope {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.pending) == 0 {
		return nil
	}

	batch := c.pending
	c.pending = make([]protocol.Envelope, 0, 64)
	return batch
}

// DrainN removes and returns up to n envelopes, oldest first, leaving the rest
// cached. Returns nil if the cache is empty.
//
// A full cache is maxSize envelopes, which as one request is megabytes compressed
// and tens of megabytes decompressed. Sending it in bounded pieces keeps any
// single request small enough for the server to enforce a meaningful body limit,
// and means a failure partway through only costs the piece that failed rather than
// the whole backlog.
func (c *metricsCache) DrainN(n int) []protocol.Envelope {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.pending) == 0 || n <= 0 {
		return nil
	}

	if n >= len(c.pending) {
		batch := c.pending
		c.pending = make([]protocol.Envelope, 0, 64)
		return batch
	}

	batch := make([]protocol.Envelope, n)
	copy(batch, c.pending[:n])

	// Shift the remainder down rather than reslicing, so the drained
	// envelopes stop being reachable and can be collected.
	remaining := copy(c.pending, c.pending[n:])
	clear(c.pending[remaining:])
	c.pending = c.pending[:remaining]

	return batch
}

// Requeue puts envelopes back at the front of the cache, where they belong by
// age. Add appends, which would make a re-queued chunk look newer than data
// collected after it and let eviction drop the wrong envelopes first.
func (c *metricsCache) Requeue(batch []protocol.Envelope) {
	if len(batch) == 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.pending = append(batch, c.pending...)

	if len(c.pending) > c.maxSize {
		c.pending = c.pending[len(c.pending)-c.maxSize:]
	}
}

func (c *metricsCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.pending)
}
