package labels

import (
	"sync"
	"testing"

	"github.com/google/uuid"
)

func TestVersionCache_FirstSightingIsDrift(t *testing.T) {
	c := NewVersionCache()
	id := uuid.NewString()

	if !c.Changed(id, "1.0.0") {
		t.Error("first sighting should report drift")
	}
}

func TestVersionCache_SameVersionNoDrift(t *testing.T) {
	c := NewVersionCache()
	id := uuid.NewString()

	c.Update(id, "1.0.0")
	if c.Changed(id, "1.0.0") {
		t.Error("identical version should not report drift")
	}
}

func TestVersionCache_DifferentVersionIsDrift(t *testing.T) {
	c := NewVersionCache()
	id := uuid.NewString()

	c.Update(id, "1.0.0")
	if !c.Changed(id, "1.0.1") {
		t.Error("changed version should report drift")
	}
}

func TestVersionCache_EmptyVersionNeverDrifts(t *testing.T) {
	c := NewVersionCache()
	id := uuid.NewString()

	// Even with no prior entry, empty version reports no drift —
	// caller has nothing actionable to sync.
	if c.Changed(id, "") {
		t.Error("empty version should not report drift on new agent")
	}

	c.Update(id, "1.0.0")
	if c.Changed(id, "") {
		t.Error("empty version should not report drift on known agent")
	}
}

func TestVersionCache_UpdateEmptyIsNoOp(t *testing.T) {
	c := NewVersionCache()
	id := uuid.NewString()

	c.Update(id, "1.0.0")
	c.Update(id, "") // should not clobber

	if c.Changed(id, "1.0.0") {
		t.Error("empty Update should not have cleared cached version")
	}
	if c.Len() != 1 {
		t.Errorf("Len = %d, want 1", c.Len())
	}
}

func TestVersionCache_DriftPersistsUntilUpdate(t *testing.T) {
	// Documents the deliberate behavior: Changed does NOT auto-update.
	// A caller that fails the sync should NOT call Update, so the next
	// request sees drift again and retries.
	c := NewVersionCache()
	id := uuid.NewString()

	if !c.Changed(id, "1.0.0") {
		t.Fatal("first Changed should return true")
	}
	if !c.Changed(id, "1.0.0") {
		t.Error("Changed should still return true until Update is called")
	}

	c.Update(id, "1.0.0")
	if c.Changed(id, "1.0.0") {
		t.Error("after Update, Changed should return false")
	}
}

func TestVersionCache_Forget(t *testing.T) {
	c := NewVersionCache()
	id := uuid.NewString()

	c.Update(id, "1.0.0")
	c.Forget(id)

	if !c.Changed(id, "1.0.0") {
		t.Error("after Forget, version should look new again")
	}
	if c.Len() != 0 {
		t.Errorf("Len = %d, want 0", c.Len())
	}
}

func TestVersionCache_ConcurrentAccess(t *testing.T) {
	// Smoke test under -race: many goroutines updating disjoint and
	// overlapping agent IDs simultaneously.
	c := NewVersionCache()
	ids := make([]string, 16)
	for i := range ids {
		ids[i] = uuid.NewString()
	}

	var wg sync.WaitGroup
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := ids[i%len(ids)]
			version := []string{"1.0.0", "1.0.1", "1.0.2"}[i%3]
			c.Changed(id, version)
			c.Update(id, version)
			c.Changed(id, version)
		}(i)
	}
	wg.Wait()

	if c.Len() == 0 || c.Len() > len(ids) {
		t.Errorf("Len = %d, want 1..%d", c.Len(), len(ids))
	}
}
