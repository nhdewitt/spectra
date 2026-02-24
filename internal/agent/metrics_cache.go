package agent

import (
	"sync"

	"github.com/nhdewitt/spectra/internal/protocol"
)

const defaultMaxCacheSize = 1000

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

func (c *metricsCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.pending)
}
