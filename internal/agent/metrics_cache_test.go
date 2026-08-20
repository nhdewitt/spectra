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
	if got[0].Hostname != "test-host-0" || got[3].Hostname != "test-host-3" {
		t.Errorf("drained the wrong envelopes: %q..%q", got[0].Hostname, got[3].Hostname)
	}
	if c.Len() != 6 {
		t.Errorf("remaining: got %d, want 6", c.Len())
	}

	next := c.DrainN(4)
	if next[0].Hostname != "test-host-4" {
		t.Errorf("second chunk starts at %q, want test-host-4", next[0].Hostname)
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
	if all[0].Hostname != "test-host-0" {
		t.Errorf("requeued envelopes are not at the front: first is %q, want test-host-0", all[0].Hostname)
	}
	if all[5].Hostname != "test-host-5" {
		t.Errorf("order after requeue is wrong: last is %q, want test-host-5", all[5].Hostname)
	}
}

func TestRequeue_EvictsOldestOnOverflow(t *testing.T) {
	c := newMetricsCache(5)
	c.Add(numberedEnvelopes(5))

	// Requeueing more than the cache can hold drops the oldest, which are the
	// ones at the front of the requeued block.
	older := []protocol.Envelope{{Hostname: "test-host-older"}, {Hostname: "test-host-older-2"}}
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
