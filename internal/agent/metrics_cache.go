package agent

import (
	"sync"

	"github.com/nhdewitt/spectra/internal/protocol"
)

const (
	// defaultMaxCacheSize is the envelope-count fallback, used only when
	// the byte ceiling is unknown. Normally, the count cap is derived from
	// the byte budget by cacheSizeFor, so it can never bind before the
	// byte ceiling.
	defaultMaxCacheSize = 20_000

	// defaultMaxCacheBytes is the byte ceiling used when the caller does
	// not derive one from host RAM. see cacheBytesFor.
	defaultMaxCacheBytes int64 = 16 << 20

	// cacheRAMDivisor is the fraction of total physical memory the cache
	// may occupy, and minCacheBytes/maxCacheBytes clamp the result.
	//
	// Sized so a full cache leaves the GOGC=100 heap target under the soft
	// memory limit set by applyMemoryLimit, which uses memLimitDivisor.
	// cacheRAMDivisor should stay at roughly 4x memLimitDivisor.
	//
	// The divisor only does work between 64MiB and 2GiB of RAM. Every host
	// with more than 2GiB gets the same cache, which is deliberate. Past
	// that point, the constraint isn't the agent's memory but the cost of
	// replaying the backlog. The clamp is ~175k envelopes, roughly 45s of
	// blocked sender and 350 batch transactions per agent as recovery.
	cacheRAMDivisor       = 16
	minCacheBytes   int64 = 4 << 20
	maxCacheBytes   int64 = 128 << 20
)

// Approximate resident cost of a cached envelope, used for the byte ceiling.
//
// Only the list types are broken out, because they are the only ones whose
// size varies by more than a small constant factor.
//
// container_list is the one list type that does not collapse, so it is the
// only payload that accumulates over an outage and the only reason the byte
// ceiling can bind before the count cap.
//
// envelopeOverheadBytes doubles as the floor used by cacheSizeFor, so it must
// stay a genuine lower bound on what one envelope costs.
const (
	envelopeOverheadBytes   int64 = 768
	processEntryBytes       int64 = 480
	serviceEntryBytes       int64 = 672
	containerEntryBytes     int64 = 864
	applicationEntryBytes   int64 = 480
	updatePackageEntryBytes int64 = 384
)

// collapsibleTypes are the metric types the server writes as current state
// rather than history. Their handlers in persist.go are upserts against a
// table with no history behind it, so replaying a backlog of them rewrites
// the same rows N times and only the last one survives. Keeping just the
// newest of each per outage costs nothing in fidelity and reclaims the
// majority of the cache's bytes, since these are the only envelopes carrying
// large slices.
//
// container_list is deliberately absent. It looks like the others, but each
// entry goes through InsertContainer into the metrics_container hypertable,
// so it is real history and collapsing it would silently delete container
// metrics for the whole outage window.
var collapsibleTypes = map[string]struct{}{
	"process_list":     {},
	"service_list":     {},
	"application_list": {},
	"updates":          {},
}

// metricsCache holds unsent metric envelopes for retry when the server
// is unreachable. Uses a bounded buffer and an approximate byte budget
// so a host with large process lists cannot outgrow its memory.
//
// Live envelopes are pending[head:]. This is a FIFO over a growable slice.
//
// Add and Requeue push, DrainN pops, eviction discards. All in age order,
// all from the head except Add. Draining advances head rather than shifting
// the remainder down, which keeps DrainN linear in the chunk instead of
// quadratic across a full drain, and lets Requeue put a failed chunk back
// into the slots it came from with no allocation. Space below head is
// reclaimed by compact once it is worth the copy.
//
// Because drain and eviction work the same way, they agree. An envelope
// evicted under pressure was the next one due to be sent. Draining from the
// tail instead would let envelopes in the middle age out having never been
// sent whenever intake approached the drain rate.
type metricsCache struct {
	mu       sync.Mutex
	pending  []protocol.Envelope
	head     int
	bytes    int64
	maxSize  int
	maxBytes int64

	// latest maps a collapsible type to the index in pending holding its
	// newest envelope, so Add can overwrite in place instead of scanning.
	latest map[string]int
}

func newMetricsCache(maxSize int) *metricsCache {
	return newMetricsCacheWithLimits(maxSize, defaultMaxCacheBytes)
}

// newMetricsCacheForBytes builds a cache from a byte budget alone, deriving
// the count backstop from it. This is what the agent uses, the count-first
// constructors exist for tests that want an exact envelope count.
func newMetricsCacheForBytes(maxBytes int64) *metricsCache {
	return newMetricsCacheWithLimits(cacheSizeFor(maxBytes), maxBytes)
}

// newMetricsCacheWithLimits builds a cache with an explicit byte ceiling.
// A maxBytes of zero or less disables the byte ceiling and leaves only
// the count cap.
func newMetricsCacheWithLimits(maxSize int, maxBytes int64) *metricsCache {
	if maxSize <= 0 {
		maxSize = defaultMaxCacheSize
	}
	return &metricsCache{
		pending:  make([]protocol.Envelope, 0, 64),
		maxSize:  maxSize,
		maxBytes: maxBytes,
		latest:   make(map[string]int, len(collapsibleTypes)),
	}
}

// cacheBytesFor derives a cache byte ceiling from total physical memory.
// A memTotal of zero means the platform read failed, in which case the
// caller gets the fixed default rather than a number derived from nothing.
func cacheBytesFor(memTotal uint64) int64 {
	if memTotal == 0 {
		return defaultMaxCacheBytes
	}

	limit := int64(memTotal / cacheRAMDivisor)
	return min(max(limit, minCacheBytes), maxCacheBytes)
}

// cacheSizeFor derives the envelope-count cap from the byte ceiling.
//
// Rounds up. Truncating would leave the count binding first whenever
// maxBytes is not a multiple of envelopeOverheadBytes. The cost is at most
// one envelope of headroom.
func cacheSizeFor(maxBytes int64) int {
	if maxBytes <= 0 {
		return defaultMaxCacheSize
	}
	return int((maxBytes + envelopeOverheadBytes - 1) / envelopeOverheadBytes)
}

// envelopeSize approximates the resident cost of one envelope.
//
// Agent-side collectors emit most metrics by value and application_list by
// pointer, so both forms are matched. Anything not listed falls through to
// the flat overhead, which is what the scalar metrics cost within a small
// factor.
func envelopeSize(e protocol.Envelope) int64 {
	n := envelopeOverheadBytes

	switch m := e.Data.(type) {
	case protocol.ProcessListMetric:
		n += int64(len(m.Processes)) * processEntryBytes
	case *protocol.ProcessListMetric:
		n += int64(len(m.Processes)) * processEntryBytes

	case protocol.ServiceListMetric:
		n += int64(len(m.Services)) * serviceEntryBytes
	case *protocol.ServiceListMetric:
		n += int64(len(m.Services)) * serviceEntryBytes

	case protocol.ContainerListMetric:
		n += int64(len(m.Containers)) * containerEntryBytes
	case *protocol.ContainerListMetric:
		n += int64(len(m.Containers)) * containerEntryBytes

	case protocol.ApplicationListMetric:
		n += int64(len(m.Applications)) * applicationEntryBytes
	case *protocol.ApplicationListMetric:
		n += int64(len(m.Applications)) * applicationEntryBytes

	case protocol.UpdateMetric:
		n += int64(len(m.Packages)) * updatePackageEntryBytes
	case *protocol.UpdateMetric:
		n += int64(len(m.Packages)) * updatePackageEntryBytes
	}

	return n
}

// live reports the number of cached envelopes. Callers must hold c.mu.
func (c *metricsCache) live() int {
	return len(c.pending) - c.head
}

// Add appends envelopes to the cache, replacing the previous envelope of
// any collapsible type in place rather than accumulating duplicates.
// Envelopes are evicted oldest-first if either limit is exceeded.
func (c *metricsCache) Add(batch []protocol.Envelope) {
	if len(batch) == 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	for i := range batch {
		c.addOne(batch[i])
	}
	c.evict()
}

// addOne inserts a single envelope. Callers must hold c.mu.
func (c *metricsCache) addOne(e protocol.Envelope) {
	if _, ok := collapsibleTypes[e.Type]; ok {
		if idx, seen := c.latest[e.Type]; seen && idx >= c.head && idx < len(c.pending) {
			c.bytes -= envelopeSize(c.pending[idx])
			c.pending[idx] = e
			c.bytes += envelopeSize(e)
			return
		}

		c.pending = append(c.pending, e)
		c.latest[e.Type] = len(c.pending) - 1
		c.bytes += envelopeSize(e)
		return
	}

	c.pending = append(c.pending, e)
	c.bytes += envelopeSize(e)
}

// evict drops the oldest envelope until the cache is within both
// limits, then reclaims dead space if it has grown large enough
// to be worth the copy. Callers must hold c.mu.
func (c *metricsCache) evict() {
	for c.live() > c.maxSize {
		c.dropOldest()
	}

	for c.maxBytes > 0 && c.bytes > c.maxBytes && c.live() > 1 {
		c.dropOldest()
	}

	c.pruneLatest()
	c.compact()
}

// dropOldest removes the envelope at head. Callers must hold c.mu.
func (c *metricsCache) dropOldest() {
	c.bytes -= envelopeSize(c.pending[c.head])
	c.pending[c.head] = protocol.Envelope{}
	c.head++
}

// pruneLatest forgets collapse targets that are no longer live.
// Callers must hold c.mu.
func (c *metricsCache) pruneLatest() {
	for t, idx := range c.latest {
		if idx < c.head {
			delete(c.latest, t)
		}
	}
}

// compact shifts live envelopes back to the front once dead space is
// at least half the slice, so the backing array does not grow without
// bound during a long outage. The capacity is kept, only the length
// shrinks. Callers must hold c.mu.
func (c *metricsCache) compact() {
	if c.head == 0 || c.head*2 < len(c.pending) {
		return
	}

	n := copy(c.pending, c.pending[c.head:])
	clear(c.pending[n:])
	c.pending = c.pending[:n]

	for t, idx := range c.latest {
		c.latest[t] = idx - c.head
	}
	c.head = 0
}

// reset enpties the cache, keeping the backing array. Callers must
// hold c.mu.
func (c *metricsCache) reset() {
	clear(c.pending)
	c.pending = c.pending[:0]
	c.head = 0
	c.bytes = 0
	clear(c.latest)
}

// Drain returns all cached envelopes and clears the cache.
// Returns nil if the cache is empty.
//
// This has no production callers, uploadBatch uses DrainN. A full cache as
// one request would exceed the server's body limit, so wiring this into the
// send path would reintroduce that.
func (c *metricsCache) Drain() []protocol.Envelope {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.live() == 0 {
		return nil
	}

	batch := make([]protocol.Envelope, c.live())
	copy(batch, c.pending[c.head:])
	c.reset()

	return batch
}

// DrainN removes and returns up to n envelopes, oldest first, leaving the rest
// cached. Returns nil if the cache is empty.
//
// Oldest-first keeps the whole send path a single FIFO. uploadBatch adds the
// current batch before draining, so during a backlog the current batch queues
// behind it, and once the backlog clears, the thing being drained is the current
// batch.
//
// current_metrics has no timestamp guard and is last-written-wins, so out-of-order
// delivery would leave the current-state tiles showing values from the middle of
// the outage. It also means that drain and eviction both work the head and agree.
// The cost is that live data is delayed behind the backlog, by roughly
// backlog/(maxUploadChunk - intake) send cycles (~75s for a 1h outage and ~30m
// for a full cache).
//
// A full cache is maxSize envelopes, which as one request is megabytes compressed
// and tens of megabytes decompressed. Sending it in bounded pieces keeps any single
// request small enough for the server to enforce a meaningful body limit, and means
// a failure partway through only costs the piece that failed rather than the whole
// backlog.
//
// This deliberately does not compact. The slots just vacated are what lets Requeue
// put a failed chunk back without allocating, and uploadBatch calls Requeue
// immediately on failure. Compaction happens on the next Add.
func (c *metricsCache) DrainN(n int) []protocol.Envelope {
	c.mu.Lock()
	defer c.mu.Unlock()

	live := c.live()
	if live == 0 || n <= 0 {
		return nil
	}
	n = min(n, live)

	batch := make([]protocol.Envelope, n)
	copy(batch, c.pending[c.head:c.head+n])

	// Clear the drained slots so their payloads stop being reachable. The
	// envelope headers stay until compact or reset reclaims them.
	for i := c.head; i < c.head+n; i++ {
		c.bytes -= envelopeSize(c.pending[i])
		c.pending[i] = protocol.Envelope{}
	}
	c.head += n

	if c.live() == 0 {
		c.reset()
		return batch
	}

	c.pruneLatest()
	return batch
}

// Requeue puts envelopes back at the front of the cache, where they belong by
// age. Add appends, which would make a re-queued chunk look newer than data
// collected after it and let eviction drop the wrong envelopes first.
//
// The common case is undoing the DrainN immediately before it, so the slots are
// still reserved below head and the envelopes go back where they were with no
// allocation and no copy of the rest of the cache.
//
// Collapse targets in the chunk are re-registered, because a chunk that emptied
// the cache carries them. DrainN resets when it drains the last envelope, and
// reset clears latest.
func (c *metricsCache) Requeue(batch []protocol.Envelope) {
	if len(batch) == 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.head >= len(batch) {
		c.head -= len(batch)
		copy(c.pending[c.head:], batch)
	} else {
		// More envelopes than reserved space - rebuild.
		grown := make([]protocol.Envelope, 0, len(batch)+c.live())
		grown = append(grown, batch...)
		grown = append(grown, c.pending[c.head:]...)

		delta := len(batch) - c.head
		for t, idx := range c.latest {
			c.latest[t] = idx + delta
		}

		c.pending = grown
		c.head = 0
	}

	for i := range batch {
		c.bytes += envelopeSize(batch[i])
	}

	// Re-register the collapse targets this chunk carries.
	tail := c.head + len(batch)
	for i := range batch {
		t := batch[i].Type
		if _, ok := collapsibleTypes[t]; !ok {
			continue
		}
		if idx, seen := c.latest[t]; seen && idx >= tail {
			continue
		}
		c.latest[t] = c.head + i
	}

	c.evict()
}

// Len reports the number of cached envelopes.
func (c *metricsCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.live()
}

// Bytes reports the cache's approximate resident cost, as counted against
// the byte ceiling. Logged next to the process heap so the estimate stays
// checkable against reality.
func (c *metricsCache) Bytes() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.bytes
}
