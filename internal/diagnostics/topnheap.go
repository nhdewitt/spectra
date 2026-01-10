package diagnostics

import (
	"cmp"
	"container/heap"
	"slices"
	"strings"

	"github.com/nhdewitt/spectra/internal/protocol"
)

type topNHeap []protocol.TopEntry

func (h topNHeap) Len() int {
	return len(h)
}

// Less implements Min-Heap.
func (h topNHeap) Less(i, j int) bool {
	if h[i].Size != h[j].Size {
		return h[i].Size < h[j].Size
	}
	return strings.ToLower(h[i].Path) > strings.ToLower(h[j].Path)
}

func (h topNHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *topNHeap) Push(x any) {
	*h = append(*h, x.(protocol.TopEntry))
}

func (h *topNHeap) Pop() any {
	old := *h
	n := old.Len()
	x := old[n-1]
	*h = old[:n-1]
	return x
}

// pushTopN maintains a heap of fixed size N.
func pushTopN(h *topNHeap, n int, e protocol.TopEntry) {
	if n <= 0 {
		return
	}
	// Keep filling until N items in heap
	if h.Len() < n {
		heap.Push(h, e)
		return
	}

	// Only replace if new item is larger than smallest
	smallest := (*h)[0]
	if e.Size > smallest.Size || (e.Size == smallest.Size && strings.ToLower(e.Path) < strings.ToLower(smallest.Path)) {
		(*h)[0] = e
		heap.Fix(h, 0)
	}
}

// popAllSortedDesc flattens the heap into a sorted slice (Largest -> Smallest; alphabetical tie-breaker).
func popAllSortedDesc(h *topNHeap) []protocol.TopEntry {
	out := make([]protocol.TopEntry, 0, h.Len())
	for h.Len() > 0 {
		out = append(out, heap.Pop(h).(protocol.TopEntry))
	}

	slices.SortFunc(out, func(a, b protocol.TopEntry) int {
		if a.Size != b.Size {
			return cmp.Compare(b.Size, a.Size)
		}
		return cmp.Compare(strings.ToLower(a.Path), strings.ToLower(b.Path))
	})

	return out
}
