package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

// --- Registration ---

func TestHandleAgentRegister_Success(t *testing.T) {
	s := New(Config{Port: 8080}, NewMockDB())
	token := s.Tokens.Generate(24 * time.Hour)

	regReq := protocol.RegisterRequest{
		Token: token,
		Info: protocol.HostInfo{
			Hostname: "new-agent",
			OS:       "linux",
			Platform: "ubuntu",
			Arch:     "amd64",
			CPUCores: 4,
		},
	}

	body, _ := json.Marshal(regReq)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status: got %d, want 201", rec.Code)
	}

	var resp protocol.RegisterResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.AgentID == "" {
		t.Error("AgentID should not be empty")
	}
	if resp.Secret == "" {
		t.Error("Secret should not be empty")
	}

	if !s.Store.Exists(resp.AgentID) {
		t.Error("agent should be registered in store")
	}
}

func TestHandleAgentRegister_InvalidToken(t *testing.T) {
	s := New(Config{Port: 8080}, NewMockDB())

	regReq := protocol.RegisterRequest{
		Token: "invalid-token",
		Info:  protocol.HostInfo{Hostname: "new-agent"},
	}

	body, _ := json.Marshal(regReq)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
}

func TestHandleAgentRegister_ExpiredToken(t *testing.T) {
	s := New(Config{Port: 8080}, NewMockDB())
	token := s.Tokens.Generate(1 * time.Nanosecond)
	time.Sleep(2 * time.Millisecond)

	regReq := protocol.RegisterRequest{
		Token: token,
		Info:  protocol.HostInfo{Hostname: "new-agent"},
	}

	body, _ := json.Marshal(regReq)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
}

func TestHandleAgentRegister_TokenSingleUse(t *testing.T) {
	s := New(Config{Port: 8080}, NewMockDB())
	token := s.Tokens.Generate(24 * time.Hour)

	regReq := protocol.RegisterRequest{
		Token: token,
		Info:  protocol.HostInfo{Hostname: "agent-1"},
	}
	body, _ := json.Marshal(regReq)

	// First use succeeds
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("first use: got %d, want 201", rec.Code)
	}

	// Second use fails
	req = httptest.NewRequest(http.MethodPost, "/api/v1/agent/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("second use: got %d, want 401", rec.Code)
	}
}

func TestHandleAgentRegister_MethodNotAllowed(t *testing.T) {
	s := New(Config{Port: 8080}, NewMockDB())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agent/register", nil)
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status: got %d, want 405", rec.Code)
	}
}

func TestHandleAgentRegister_InvalidJSON(t *testing.T) {
	s := New(Config{Port: 8080}, NewMockDB())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/register", bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

// --- Auth Middleware ---

func TestRequireAgentAuth_Success(t *testing.T) {
	s, agentID, secret, _ := newTestServer()

	batch := []RawEnvelope{}
	body, _ := json.Marshal(batch)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/metrics", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	setAgentAuth(req, agentID, secret)
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code == http.StatusUnauthorized {
		t.Error("should not be 401 with valid credentials")
	}
}

func TestRequireAgentAuth_MissingHeaders(t *testing.T) {
	s, _, _, _ := newTestServer()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/metrics", bytes.NewReader([]byte("[]")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
}

func TestRequireAgentAuth_WrongSecret(t *testing.T) {
	s, agentID, _, _ := newTestServer()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/metrics", bytes.NewReader([]byte("[]")))
	req.Header.Set("Content-Type", "application/json")
	setAgentAuth(req, agentID, "wrong-secret")
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
}

func TestRequireAgentAuth_UnknownAgent(t *testing.T) {
	s, _, _, _ := newTestServer()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/metrics", bytes.NewReader([]byte("[]")))
	req.Header.Set("Content-Type", "application/json")
	setAgentAuth(req, "nonexistent", "any-secret")
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
}

// --- Metrics ---

func TestHandleMetrics_Success(t *testing.T) {
	s, agentID, secret, _ := newTestServer()

	batch := []RawEnvelope{
		{
			Type:     "cpu",
			Hostname: "test-host",
			Data:     json.RawMessage(`{"usage": 50.0}`),
		},
	}

	body, _ := json.Marshal(batch)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/metrics", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	setAgentAuth(req, agentID, secret)
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Errorf("status: got %d, want 202", rec.Code)
	}
}

func TestHandleMetrics_EmptyBatch(t *testing.T) {
	s, agentID, secret, _ := newTestServer()

	body, _ := json.Marshal([]RawEnvelope{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/metrics", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	setAgentAuth(req, agentID, secret)
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code >= 500 {
		t.Errorf("status: got %d, should not be 5xx for empty batch", rec.Code)
	}
}

func TestHandleMetrics_InvalidJSON(t *testing.T) {
	s, agentID, secret, _ := newTestServer()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/metrics", bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	setAgentAuth(req, agentID, secret)
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

// --- Agent Command ---

func TestHandleAgentCommand_NoCommands(t *testing.T) {
	s, agentID, secret, _ := newTestServer()
	s.Config.CommandTimeout = 10 * time.Millisecond

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agent/command", nil)
	setAgentAuth(req, agentID, secret)
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK && rec.Code != http.StatusNoContent {
		t.Errorf("status: got %d, want 200 or 204", rec.Code)
	}
}

func TestHandleAgentCommand_WithCommand(t *testing.T) {
	s, agentID, secret, _ := newTestServer()
	s.Store.QueueCommand(agentID, protocol.Command{
		ID:   "cmd-123",
		Type: protocol.CmdFetchLogs,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agent/command", nil)
	setAgentAuth(req, agentID, secret)
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}

	var cmd protocol.Command
	if err := json.NewDecoder(rec.Body).Decode(&cmd); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if cmd.ID != "cmd-123" {
		t.Errorf("command ID: got %s, want cmd-123", cmd.ID)
	}
}

func TestHandleAgentCommand_NoAuth(t *testing.T) {
	s, _, _, _ := newTestServer()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agent/command", nil)
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
}

// --- Command Result ---

func TestHandleCommandResult_Success(t *testing.T) {
	s, agentID, secret, _ := newTestServer()

	logs := []protocol.LogEntry{
		{
			Timestamp: time.Now().Unix(),
			Source:    "test-host",
			Level:     protocol.LevelInfo,
			Message:   "Test log message",
		},
	}
	logBytes, _ := json.Marshal(logs)

	result := protocol.CommandResult{
		ID:      "cmd-123",
		Type:    protocol.CmdFetchLogs,
		Payload: json.RawMessage(logBytes),
	}

	body, _ := json.Marshal(result)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/command/result", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	setAgentAuth(req, agentID, secret)
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestHandleCommandResult_WithError(t *testing.T) {
	s, agentID, secret, _ := newTestServer()

	result := protocol.CommandResult{
		ID:    "cmd-123",
		Type:  protocol.CmdFetchLogs,
		Error: "permission denied",
	}

	body, _ := json.Marshal(result)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/command/result", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	setAgentAuth(req, agentID, secret)
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestHandleCommandResult_InvalidJSON(t *testing.T) {
	s, agentID, secret, _ := newTestServer()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/command/result", bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	setAgentAuth(req, agentID, secret)
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestHandleCommandResult_NoAuth(t *testing.T) {
	s, _, _, _ := newTestServer()

	result := protocol.CommandResult{ID: "cmd-123", Type: protocol.CmdFetchLogs}
	body, _ := json.Marshal(result)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/command/result", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
}

// --- Admin Triggers ---

func TestHandleAdminTriggerLogs_Success(t *testing.T) {
	s, agentID, _, _ := newTestServer()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/logs?agent="+agentID, nil)
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestHandleAdminTriggerLogs_MissingAgent(t *testing.T) {
	s := New(Config{Port: 8080}, NewMockDB())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/logs", nil)
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestHandleAdminTriggerLogs_UnknownAgent(t *testing.T) {
	s := New(Config{Port: 8080}, NewMockDB())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/logs?agent=nonexistent", nil)
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", rec.Code)
	}
}

func TestHandleAdminTriggerDisk_Success(t *testing.T) {
	s, agentID, _, _ := newTestServer()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/disk?agent="+agentID+"&path=/&top_n=10", nil)
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestHandleAdminTriggerDisk_MissingAgent(t *testing.T) {
	s := New(Config{Port: 8080}, NewMockDB())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/disk", nil)
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestHandleAdminTriggerNetwork_Success(t *testing.T) {
	s, agentID, _, _ := newTestServer()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/network?agent="+agentID+"&action=ping&target=8.8.8.8", nil)
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestHandleAdminTriggerNetwork_MissingAgent(t *testing.T) {
	s := New(Config{Port: 8080}, NewMockDB())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/network?action=ping", nil)
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestHandleAdminTriggerNetwork_MissingAction(t *testing.T) {
	s, agentID, _, _ := newTestServer()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/network?agent="+agentID, nil)
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

// --- Token Generation ---

func TestHandleGenerateToken(t *testing.T) {
	s := New(Config{Port: 8080}, NewMockDB())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/tokens", nil)
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status: got %d, want 201", rec.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if resp["token"] == "" {
		t.Error("token should not be empty")
	}
}
