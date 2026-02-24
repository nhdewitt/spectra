package agent

import (
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
