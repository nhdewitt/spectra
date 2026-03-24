package server

import (
	"crypto/sha256"
	"errors"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var errFake = errors.New("fake error")

// newTestServer creates a server with a MockDB and returns it along with
// a registered agent's ID, plaintext secret, and the mock.
func newTestServer() (*Server, string, string, *MockDB) {
	mock := NewMockDB()

	s := New(Config{Port: 8080, CommandTimeout: 10 * time.Millisecond}, mock)

	agentID := "550e8400-e29b-41d4-a716-446655440000"
	secret := "test-secret"

	hash, _ := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	mock.Agents[agentID] = string(hash)

	sum := sha256.Sum256([]byte(secret))
	mock.AgentSHA256[agentID] = sum[:]

	return s, agentID, secret, mock
}

// setAgentAuth sets the auth headers on a request.
func setAgentAuth(req *http.Request, agentID, secret string) {
	req.Header.Set("X-Agent-ID", agentID)
	req.Header.Set("X-Agent-Secret", secret)
}
