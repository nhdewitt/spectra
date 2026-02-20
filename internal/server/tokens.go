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

func (ts *TokenStore) Generate(ttl time.Duration) string {
	token := uuid.New().String()
	now := time.Now()

	ts.mu.Lock()
	defer ts.mu.Unlock()

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

	t, ok := ts.tokens[token]
	if !ok || t.Used || time.Now().After(t.ExpiresAt) {
		return false
	}

	t.Used = true
	return true
}
