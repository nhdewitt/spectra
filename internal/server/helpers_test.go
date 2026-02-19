package server

import (
	"net/http"

	"github.com/nhdewitt/spectra/internal/protocol"
	"golang.org/x/crypto/bcrypt"
)

// newTestServer creates a server with a MockDB and returns it along with
// a registered agent's ID, plaintext secret, and the mock.
func newTestServer() (*Server, string, string, *MockDB) {
	mock := NewMockDB()

	s := New(Config{Port: 8080}, mock)

	agentID := "550e8400-e29b-41d4-a716-446655440000"
	secret := "test-secret"

	hash, _ := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	mock.Agents[agentID] = string(hash)

	s.Store.Register(agentID, secret, protocol.HostInfo{
		Hostname: "test-host",
		OS:       "linux",
	})

	return s, agentID, secret, mock
}

// setAgentAuth sets the auth headers on a request.
func setAgentAuth(req *http.Request, agentID, secret string) {
	req.Header.Set("X-Agent-ID", agentID)
	req.Header.Set("X-Agent-Secret", secret)
}

// registerTestAgent is a shorthand for registering a test agent in the store.
func registerTestAgent(store *AgentStore, agentID string) {
	store.Register(agentID, "secret-"+agentID, protocol.HostInfo{
		Hostname: "host-" + agentID,
		OS:       "linux",
	})
}
