package server

import (
	"sync"
	"testing"
	"time"
)

func TestTokenStore_GenerateAndValidate(t *testing.T) {
	ts := NewTokenStore()
	token := ts.Generate(time.Minute)

	if !ts.Peek(token) {
		t.Error("fresh token should peek valid")
	}
	if !ts.Validate(token) {
		t.Error("fresh token should validate")
	}
	if ts.Validate(token) {
		t.Error("token validated twice")
	}
}

func TestTokenStore_RejectsExpired(t *testing.T) {
	ts := NewTokenStore()
	token := ts.Generate(-time.Second)

	if ts.Peek(token) {
		t.Error("expired token should not peek valid")
	}
	if ts.Validate(token) {
		t.Error("expired token should not validate")
	}
}

func TestTokenStore_RejectsUnknown(t *testing.T) {
	ts := NewTokenStore()

	if ts.Peek("no-such-token") {
		t.Error("unknown token should not peek valid")
	}
	if ts.Validate("no-such-token") {
		t.Error("unknown token should not validate")
	}
}

// Nothing but RevokeAll used to release an entry, so a long-lived server
// accumulated every token it had ever issued.
func TestTokenStore_PrunesExpiredOnGenerate(t *testing.T) {
	ts := NewTokenStore()

	// Each Generate prunes before inserting, so the loop never accumulates;
	// only the most recent expired token survives into the next call.
	for range 10 {
		ts.Generate(-time.Second)
	}

	live := ts.Generate(time.Minute)

	if got := len(ts.tokens); got != 1 {
		t.Errorf("map holds %d tokens, want 1", got)
	}
	if !ts.Peek(live) {
		t.Error("the live token was pruned")
	}
}

func TestTokenStore_PrunesUsedOnGenerate(t *testing.T) {
	ts := NewTokenStore()

	spent := ts.Generate(time.Minute)
	if !ts.Validate(spent) {
		t.Fatal("token should have validated")
	}

	live := ts.Generate(time.Minute)

	if got := len(ts.tokens); got != 1 {
		t.Errorf("map holds %d tokens, want 1", got)
	}
	if ts.Peek(spent) {
		t.Error("spent token still peeks valid")
	}
	if !ts.Peek(live) {
		t.Error("the live token was pruned")
	}
}

// Pruning must not take out tokens that are still usable.
func TestTokenStore_PruneKeepsLiveTokens(t *testing.T) {
	ts := NewTokenStore()

	a := ts.Generate(time.Minute)
	b := ts.Generate(time.Minute)
	ts.Generate(-time.Second)
	c := ts.Generate(time.Minute)

	if got := len(ts.tokens); got != 3 {
		t.Fatalf("map holds %d tokens, want 3", got)
	}
	for _, token := range []string{a, b, c} {
		if !ts.Peek(token) {
			t.Errorf("live token %q was pruned", token)
		}
	}
}

func TestTokenStore_ValidatePrunes(t *testing.T) {
	ts := NewTokenStore()

	live := ts.Generate(time.Minute)
	for range 5 {
		ts.Generate(-time.Second)
	}

	if !ts.Validate(live) {
		t.Fatal("live token should have validated")
	}

	// The validated token stays until the next write; the expired ones go.
	if got := len(ts.tokens); got != 1 {
		t.Errorf("map holds %d tokens, want 1", got)
	}
}

func TestTokenStore_RevokeAll(t *testing.T) {
	ts := NewTokenStore()

	token := ts.Generate(time.Minute)
	ts.RevokeAll()

	if ts.Peek(token) {
		t.Error("token survived RevokeAll")
	}
	if got := len(ts.tokens); got != 0 {
		t.Errorf("map holds %d tokens after RevokeAll, want 0", got)
	}
}

// pruneLocked mutates the map from inside the same paths that read it, so the
// race detector has something to look at here.
func TestTokenStore_ConcurrentAccess(t *testing.T) {
	ts := NewTokenStore()

	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			token := ts.Generate(time.Minute)
			ts.Peek(token)
			ts.Validate(token)
		}()
	}
	wg.Wait()
}
