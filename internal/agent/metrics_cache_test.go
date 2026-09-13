package agent

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func makeEnvelopes(n int) []protocol.Envelope {
	envs := make([]protocol.Envelope, n)
	for i := range envs {
		envs[i] = protocol.Envelope{
			Type:      "cpu",
			Timestamp: time.Now(),
		}
	}
	return envs
}

func TestMetricsCache_AddAndDrain(t *testing.T) {
	c := newMetricsCache(100)

	c.Add(makeEnvelopes(5))
	if c.Len() != 5 {
		t.Errorf("Len() = %d, want 5", c.Len())
	}

	c.Add(makeEnvelopes(3))
	if c.Len() != 8 {
		t.Errorf("Len() = %d, want 8", c.Len())
	}

	batch := c.Drain()
	if len(batch) != 8 {
		t.Errorf("Drain() returned %d, want 8", len(batch))
	}
	if c.Len() != 0 {
		t.Errorf("Len() after drain = %d, want 0", c.Len())
	}
}

func TestMetricsCache_Removal(t *testing.T) {
	c := newMetricsCache(10)

	c.Add(makeEnvelopes(15))
	if c.Len() != 10 {
		t.Errorf("Len() = %d, want 10 (should remove oldest)", c.Len())
	}
}

func TestMetricsCache_RemovalKeepsNewest(t *testing.T) {
	c := newMetricsCache(5)

	// Add 3, then 5 - should keep last 5
	first := makeEnvelopes(3)
	first[0].Type = "old"
	c.Add(first)

	second := makeEnvelopes(5)
	second[4].Type = "newest"
	c.Add(second)

	batch := c.Drain()
	if len(batch) != 5 {
		t.Fatalf("Drain() returned %d, want 5", len(batch))
	}
	if batch[4].Type != "newest" {
		t.Error("newest envelope should be preserved")
	}
	if batch[0].Type == "old" {
		t.Error("oldest envelope should have been removed")
	}
}

func TestMetricsCache_DefaultMaxSize(t *testing.T) {
	c := newMetricsCache(0)
	if c.maxSize != defaultMaxCacheSize {
		t.Errorf("maxSize = %d, want %d", c.maxSize, defaultMaxCacheSize)
	}
}

func TestMetricsCache_Concurrent(t *testing.T) {
	c := newMetricsCache(1000)
	var wg sync.WaitGroup

	// Writers
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				c.Add(makeEnvelopes(1))
			}
		}()
	}

	// Readers
	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 50 {
				c.Drain()
			}
		}()
	}

	wg.Wait()
}

func BenchmarkMetricsCache_Add(b *testing.B) {
	c := newMetricsCache(1000)
	batch := makeEnvelopes(10)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		c.Add(batch)
		if c.Len() > 900 {
			c.Drain()
		}
	}
}

func BenchmarkMetricsCache_Drain(b *testing.B) {
	c := newMetricsCache(1000)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		c.Add(makeEnvelopes(50))
		c.Drain()
	}
}

// numberedEnvelopes gives each envelope a distinct hostname so order-sensitive
// assertions can tell them apart. makeEnvelopes leaves Hostname empty.
func numberedEnvelopes(n int) []protocol.Envelope {
	envs := make([]protocol.Envelope, n)
	for i := range envs {
		envs[i] = protocol.Envelope{
			Type:      "cpu",
			Timestamp: time.Now(),
			Hostname:  fmt.Sprintf("test-host-%d", i),
		}
	}
	return envs
}

func TestDrainN_TakesOldestFirst(t *testing.T) {
	c := newMetricsCache(100)
	c.Add(numberedEnvelopes(10))

	got := c.DrainN(4)
	if len(got) != 4 {
		t.Fatalf("drained: got %d, want 4", len(got))
	}
	for i, e := range got {
		if want := fmt.Sprintf("test-host-%d", i); e.Hostname != want {
			t.Errorf("position %d: got %q, want %q", i, e.Hostname, want)
		}
	}

	next := c.DrainN(4)
	if next[0].Hostname != "test-host-4" {
		t.Errorf("second chunk should continue forwards: first is %q, want test-host-4", next[0].Hostname)
	}
	if c.Len() != 2 {
		t.Errorf("remaining: got %d, want 2 (the two newest)", c.Len())
	}
}

func TestDrainN_TakesAllWhenNExceedsCache(t *testing.T) {
	c := newMetricsCache(100)
	c.Add(numberedEnvelopes(3))

	if got := c.DrainN(500); len(got) != 3 {
		t.Errorf("drained: got %d, want 3", len(got))
	}
	if c.Len() != 0 {
		t.Errorf("remaining: got %d, want 0", c.Len())
	}
}

func TestDrainN_EmptyAndNonPositive(t *testing.T) {
	c := newMetricsCache(100)

	if got := c.DrainN(10); got != nil {
		t.Errorf("empty cache: got %v, want nil", got)
	}

	c.Add(numberedEnvelopes(3))
	if got := c.DrainN(0); got != nil {
		t.Errorf("n=0: got %v, want nil", got)
	}
	if c.Len() != 3 {
		t.Errorf("n=0 must not consume: got %d, want 3", c.Len())
	}
}

func TestDrainN_LoopEmptiesCache(t *testing.T) {
	c := newMetricsCache(1000)
	c.Add(numberedEnvelopes(250))

	chunks := 0
	for {
		batch := c.DrainN(100)
		if len(batch) == 0 {
			break
		}
		chunks++
		if chunks > 10 {
			t.Fatal("DrainN loop did not terminate")
		}
	}

	if chunks != 3 {
		t.Errorf("chunks: got %d, want 3", chunks)
	}
	if c.Len() != 0 {
		t.Errorf("remaining: got %d, want 0", c.Len())
	}
}

func TestRequeue_PutsEnvelopesBackAtTheFront(t *testing.T) {
	c := newMetricsCache(100)
	c.Add(numberedEnvelopes(6))

	drained := c.DrainN(2)
	c.Requeue(drained)

	if c.Len() != 6 {
		t.Fatalf("cache size after requeue: got %d, want 6", c.Len())
	}

	all := c.Drain()
	for i, e := range all {
		if want := fmt.Sprintf("test-host-%d", i); e.Hostname != want {
			t.Errorf("position %d: got %q, want %q: requeue must restore the prior order", i, e.Hostname, want)
		}
	}
}

// TestRequeue_KeepsCollapseWorkingAcrossRetries covers the window where a drain
// empties the cache: DrainN resets, reset clears latest, and without Requeue
// re-registering, the next Add appends a second process_list instead of
// collapsing into the first. The cache is under maxUploadChunk for the opening
// minutes of every outage, so this is the common case, not an edge one.
func TestRequeue_KeepsCollapseWorkingAcrossRetries(t *testing.T) {
	c := newMetricsCache(100)
	c.Add(makeEnvelopes(20))
	c.Add([]protocol.Envelope{listEnvelope("process_list", "gen-0", 10)})

	for i := 1; i <= 5; i++ {
		c.Requeue(c.DrainN(maxUploadChunk))
		c.Add([]protocol.Envelope{listEnvelope("process_list", fmt.Sprintf("gen-%d", i), 10)})
	}

	lists := 0
	var newest string
	for _, e := range c.Drain() {
		if e.Type == "process_list" {
			lists++
			newest = e.Hostname
		}
	}
	if lists != 1 {
		t.Fatalf("process_list envelopes: got %d, want 1: retries orphaned collapse targets", lists)
	}
	if newest != "gen-5" {
		t.Errorf("surviving generation: got %q, want gen-5", newest)
	}
}

func TestRequeue_EvictsOldestOnOverflow(t *testing.T) {
	c := newMetricsCache(5)
	c.Add(numberedEnvelopes(5))

	// Requeue prepends, so an over-capacity requeue drops from the front — the
	// oldest of the re-queued envelopes themselves.
	older := []protocol.Envelope{{Hostname: "test-host-older-0"}, {Hostname: "test-host-older-1"}}
	c.Requeue(older)

	if c.Len() != 5 {
		t.Fatalf("cache size: got %d, want 5 (maxSize)", c.Len())
	}
	all := c.Drain()
	if all[len(all)-1].Hostname != "test-host-4" {
		t.Errorf("newest envelope was evicted: last is %q, want test-host-4", all[len(all)-1].Hostname)
	}
}

func TestRequeue_EmptyIsNoOp(t *testing.T) {
	c := newMetricsCache(100)
	c.Add(numberedEnvelopes(3))

	c.Requeue(nil)

	if c.Len() != 3 {
		t.Errorf("cache size: got %d, want 3", c.Len())
	}
}

// listEnvelope builds a collapsible list envelope with a distinguishable
// hostname, so assertions can tell which generation survived a collapse.
func listEnvelope(metricType, host string, entries int) protocol.Envelope {
	e := protocol.Envelope{
		Type:      metricType,
		Timestamp: time.Now(),
		Hostname:  host,
	}

	switch metricType {
	case "process_list":
		e.Data = protocol.ProcessListMetric{Processes: make([]protocol.ProcessMetric, entries)}
	case "service_list":
		e.Data = protocol.ServiceListMetric{Services: make([]protocol.ServiceMetric, entries)}
	case "container_list":
		e.Data = protocol.ContainerListMetric{Containers: make([]protocol.ContainerMetric, entries)}
	}

	return e
}

func TestCollapse_KeepsOnlyNewestCurrentStateEnvelope(t *testing.T) {
	c := newMetricsCache(100)

	for i := range 5 {
		c.Add([]protocol.Envelope{listEnvelope("process_list", fmt.Sprintf("gen-%d", i), 10)})
	}

	if c.Len() != 1 {
		t.Fatalf("cache size: got %d, want 1 (older process lists should collapse)", c.Len())
	}

	all := c.Drain()
	if all[0].Hostname != "gen-4" {
		t.Errorf("collapse kept the wrong generation: got %q, want gen-4", all[0].Hostname)
	}
}

func TestCollapse_DoesNotTouchTimeSeriesEnvelopes(t *testing.T) {
	c := newMetricsCache(100)

	// container_list is a list type but each entry is inserted into a
	// hypertable, so collapsing it would delete history.
	for i := range 4 {
		c.Add([]protocol.Envelope{listEnvelope("container_list", fmt.Sprintf("gen-%d", i), 3)})
	}
	c.Add(makeEnvelopes(3))

	if c.Len() != 7 {
		t.Errorf("cache size: got %d, want 7 (container_list must not collapse)", c.Len())
	}
}

func TestCollapse_IsPerType(t *testing.T) {
	c := newMetricsCache(100)

	for range 3 {
		c.Add([]protocol.Envelope{
			listEnvelope("process_list", "p", 10),
			listEnvelope("service_list", "s", 10),
		})
	}

	if c.Len() != 2 {
		t.Errorf("cache size: got %d, want 2 (one survivor per collapsible type)", c.Len())
	}
}

func TestCollapse_ReclaimsBytes(t *testing.T) {
	c := newMetricsCache(100)

	c.Add([]protocol.Envelope{listEnvelope("process_list", "first", 500)})
	big := c.Bytes()

	c.Add([]protocol.Envelope{listEnvelope("process_list", "second", 5)})
	small := c.Bytes()

	if small >= big {
		t.Errorf("collapsing a large list into a small one did not reclaim bytes: %d -> %d", big, small)
	}
}

func TestByteCeiling_EvictsOldestFirst(t *testing.T) {
	// Room for a handful of envelopes by bytes, far more by count.
	c := newMetricsCacheWithLimits(1000, envelopeOverheadBytes*4)
	c.Add(numberedEnvelopes(10))

	if c.Len() >= 10 {
		t.Fatalf("cache size: got %d, want fewer than 10 (byte ceiling should bind)", c.Len())
	}

	all := c.Drain()
	if all[len(all)-1].Hostname != "test-host-9" {
		t.Errorf("newest envelope was evicted: last is %q, want test-host-9", all[len(all)-1].Hostname)
	}
}

func TestByteCeiling_NeverEmptiesTheCache(t *testing.T) {
	c := newMetricsCacheWithLimits(1000, 1)
	c.Add([]protocol.Envelope{listEnvelope("container_list", "huge", 10_000)})

	if c.Len() != 1 {
		t.Errorf("cache size: got %d, want 1 (a single oversized envelope must survive)", c.Len())
	}
}

func TestByteCeiling_DisabledWhenNonPositive(t *testing.T) {
	c := newMetricsCacheWithLimits(50, 0)
	c.Add(numberedEnvelopes(40))

	if c.Len() != 40 {
		t.Errorf("cache size: got %d, want 40 (byte ceiling should be off)", c.Len())
	}
}

func TestBytes_ReturnsToZeroWhenDrained(t *testing.T) {
	c := newMetricsCache(100)
	c.Add(numberedEnvelopes(20))

	if c.Bytes() == 0 {
		t.Fatal("Bytes() = 0 after Add")
	}

	for c.DrainN(7) != nil {
	}

	if c.Bytes() != 0 {
		t.Errorf("Bytes() = %d after full drain, want 0", c.Bytes())
	}
	if c.Len() != 0 {
		t.Errorf("Len() = %d after full drain, want 0", c.Len())
	}
}

func TestDrainRequeueCycle_PreservesOrderAndBytes(t *testing.T) {
	c := newMetricsCache(100)
	c.Add(numberedEnvelopes(6))
	want := c.Bytes()

	// The retry path in uploadBatch: drain the oldest chunk, fail, put it back.
	for range 5 {
		drained := c.DrainN(2)
		c.Requeue(drained)
	}

	if c.Bytes() != want {
		t.Errorf("Bytes() = %d after drain/requeue cycles, want %d", c.Bytes(), want)
	}

	all := c.Drain()
	if len(all) != 6 {
		t.Fatalf("cache size: got %d, want 6", len(all))
	}
	for i, e := range all {
		if want := fmt.Sprintf("test-host-%d", i); e.Hostname != want {
			t.Errorf("position %d: got %q, want %q", i, e.Hostname, want)
		}
	}
}

func TestEnvelopeSize_ScalesWithListLength(t *testing.T) {
	small := envelopeSize(listEnvelope("process_list", "h", 10))
	large := envelopeSize(listEnvelope("process_list", "h", 100))

	if large <= small {
		t.Errorf("envelopeSize did not grow with list length: %d entries -> %d, 100 -> %d", 10, small, large)
	}

	// Pointer payloads are what the nightly application list uses.
	byValue := envelopeSize(protocol.Envelope{
		Type: "application_list",
		Data: protocol.ApplicationListMetric{Applications: make([]protocol.Application, 50)},
	})
	byPointer := envelopeSize(protocol.Envelope{
		Type: "application_list",
		Data: &protocol.ApplicationListMetric{Applications: make([]protocol.Application, 50)},
	})
	if byValue != byPointer {
		t.Errorf("value and pointer payloads sized differently: %d vs %d", byValue, byPointer)
	}
}

func TestCacheBytesFor(t *testing.T) {
	tests := []struct {
		name     string
		memTotal uint64
		want     int64
	}{
		{"unreadable falls back to the fixed default", 0, defaultMaxCacheBytes},
		{"tiny host clamps up to the floor", 32 << 20, minCacheBytes},
		{"large host clamps down to the ceiling", 64 << 30, maxCacheBytes},
		{"small host gets a derived value", 192 << 20, (192 << 20) / cacheRAMDivisor},
		{"multi-GiB host clamps to the ceiling", 4 << 30, maxCacheBytes},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cacheBytesFor(tt.memTotal); got != tt.want {
				t.Errorf("cacheBytesFor(%d) = %d, want %d", tt.memTotal, got, tt.want)
			}
		})
	}
}

func TestCacheSizeFor_NeverBindsBeforeTheByteCeiling(t *testing.T) {
	// envelopeOverheadBytes is the floor on a single envelope, so the derived
	// count must be unreachable without the byte ceiling engaging first.
	for _, maxBytes := range []int64{minCacheBytes, 16 << 20, maxCacheBytes} {
		n := cacheSizeFor(maxBytes)
		if got := int64(n) * envelopeOverheadBytes; got < maxBytes {
			t.Errorf("cacheSizeFor(%d) = %d envelopes, only %d bytes: count cap binds first",
				maxBytes, n, got)
		}
	}
}

func TestCacheSizeFor_FallsBackWithoutAByteBudget(t *testing.T) {
	if got := cacheSizeFor(0); got != defaultMaxCacheSize {
		t.Errorf("cacheSizeFor(0) = %d, want %d", got, defaultMaxCacheSize)
	}
}

func TestNewMetricsCacheForBytes_DerivesBothLimits(t *testing.T) {
	c := newMetricsCacheForBytes(16 << 20)

	if c.maxBytes != 16<<20 {
		t.Errorf("maxBytes = %d, want %d", c.maxBytes, int64(16<<20))
	}
	if want := cacheSizeFor(16 << 20); c.maxSize != want {
		t.Errorf("maxSize = %d, want %d", c.maxSize, want)
	}
}
