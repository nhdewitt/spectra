package diagnostics

import (
	"container/heap"
	"math/rand"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func TestTopNHeap(t *testing.T) {
	tests := []struct {
		name     string
		n        int
		entries  []protocol.TopEntry
		expected []protocol.TopEntry
	}{
		{
			name: "basic top 3",
			n:    3,
			entries: []protocol.TopEntry{
				{Path: "a", Size: 100},
				{Path: "b", Size: 300},
				{Path: "c", Size: 200},
			},
			expected: []protocol.TopEntry{
				{Path: "b", Size: 300},
				{Path: "c", Size: 200},
				{Path: "a", Size: 100},
			},
		},
		{
			name: "more entries than n",
			n:    2,
			entries: []protocol.TopEntry{
				{Path: "a", Size: 100},
				{Path: "b", Size: 300},
				{Path: "c", Size: 200},
				{Path: "d", Size: 50},
			},
			expected: []protocol.TopEntry{
				{Path: "b", Size: 300},
				{Path: "c", Size: 200},
			},
		},
		{
			name: "tie-breaker by path",
			n:    3,
			entries: []protocol.TopEntry{
				{Path: "Zebra", Size: 100},
				{Path: "apple", Size: 100},
				{Path: "Banana", Size: 100},
			},
			expected: []protocol.TopEntry{
				{Path: "apple", Size: 100},
				{Path: "Banana", Size: 100},
				{Path: "Zebra", Size: 100},
			},
		},
		{
			name:     "n is zero",
			n:        0,
			entries:  []protocol.TopEntry{{Path: "a", Size: 100}},
			expected: []protocol.TopEntry{},
		},
		{
			name: "fewer entries than n",
			n:    5,
			entries: []protocol.TopEntry{
				{Path: "a", Size: 100},
				{Path: "b", Size: 200},
			},
			expected: []protocol.TopEntry{
				{Path: "b", Size: 200},
				{Path: "a", Size: 100},
			},
		},
		{
			name: "replacement at boundary",
			n:    2,
			entries: []protocol.TopEntry{
				{Path: "c", Size: 100},
				{Path: "b", Size: 100},
				{Path: "a", Size: 100},
			},
			expected: []protocol.TopEntry{
				{Path: "a", Size: 100},
				{Path: "b", Size: 100},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &topNHeap{}
			heap.Init(h)

			for _, e := range tt.entries {
				pushTopN(h, tt.n, e)
			}

			got := popAllSortedDesc(h)

			if len(got) != len(tt.expected) {
				t.Fatalf("length mismatch: got %d, want %d", len(got), len(tt.expected))
			}

			for i := range got {
				if got[i].Path != tt.expected[i].Path || got[i].Size != tt.expected[i].Size {
					t.Errorf("index %d: got %+v, want %+v", i, got[i], tt.expected[i])
				}
			}
		})
	}
}

// Naive topNSort for benchmarking comparison.
func topNSort(entries []protocol.TopEntry, n int) []protocol.TopEntry {
	slices.SortFunc(entries, func(a, b protocol.TopEntry) int {
		if a.Size != b.Size {
			if b.Size > a.Size {
				return 1
			}
			return -1
		}
		return strings.Compare(strings.ToLower(a.Path), strings.ToLower(b.Path))
	})

	if len(entries) < n {
		return entries
	}
	return entries[:n]
}

// Heap approach wrapper for fair comparison
func topNHeapApproach(entries []protocol.TopEntry, n int) []protocol.TopEntry {
	h := make(topNHeap, 0, n)
	heap.Init(&h)

	for _, e := range entries {
		pushTopN(&h, n, e)
	}

	return popAllSortedDesc(&h)
}

func generateEntries(count int) []protocol.TopEntry {
	entries := make([]protocol.TopEntry, count)
	for i := range entries {
		entries[i] = protocol.TopEntry{
			Path: "/path/to/file" + strconv.Itoa(i),
			Size: uint64(rand.Int63n(1_000_000_000)),
		}
	}
	return entries
}

func BenchmarkTopN(b *testing.B) {
	sizes := []int{100, 1_000, 10_000, 100_000}
	n := 10

	for _, size := range sizes {
		entries := generateEntries(size)

		b.Run("heap/"+strconv.Itoa(size), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				// Copy since heap modifies state
				e := make([]protocol.TopEntry, len(entries))
				copy(e, entries)
				topNHeapApproach(e, n)
			}
		})

		b.Run("sort/"+strconv.Itoa(size), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				// Copy since sort is in-place
				e := make([]protocol.TopEntry, len(entries))
				copy(e, entries)
				topNSort(e, n)
			}
		})
	}
}
