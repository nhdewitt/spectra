package server

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

type RegistrationToken struct {
	Token     string
	CreatedAt time.Time
	ExpiresAt time.Time
	Used      bool
}

type TokenStore struct {
	mu     sync.Mutex
	tokens map[string]*RegistrationToken
}

func NewTokenStore() *TokenStore {
	return &TokenStore{
		tokens: make(map[string]*RegistrationToken),
	}
}

// pruneLocked drops tokens that can never succeed again. Generation
// is admin-only and rate limited, so the map is small and a full scan
// on each write is cheaper than a background sweeper. Without it,
// nothing but RevokeAll ever releases an entry and the map grows for
// the life of the process.
//
// Callers must hold ts.mu.
func (ts *TokenStore) pruneLocked(now time.Time) {
	for k, t := range ts.tokens {
		if t.Used || now.After(t.ExpiresAt) {
			delete(ts.tokens, k)
		}
	}
}

func (ts *TokenStore) Generate(ttl time.Duration) string {
	token := uuid.New().String()
	now := time.Now()

	ts.mu.Lock()
	defer ts.mu.Unlock()

	ts.pruneLocked(now)

	ts.tokens[token] = &RegistrationToken{
		Token:     token,
		CreatedAt: now,
		ExpiresAt: now.Add(ttl),
	}

	return token
}

func (ts *TokenStore) Validate(token string) bool {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	// Pruning first rather than last leaves the token being validated
	// in the map until the next write, so Peek behavior is unchanged
	// within a call.
	now := time.Now()
	ts.pruneLocked(now)

	t, ok := ts.tokens[token]
	if !ok || t.Used || now.After(t.ExpiresAt) {
		return false
	}

	t.Used = true
	return true
}

func (ts *TokenStore) RevokeAll() {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.tokens = make(map[string]*RegistrationToken)
}

// Peek checks if a token is valid without consuming it.
func (ts *TokenStore) Peek(token string) bool {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	t, ok := ts.tokens[token]
	if !ok {
		return false
	}

	return !t.Used && time.Now().Before(t.ExpiresAt)
}
