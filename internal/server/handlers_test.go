package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nhdewitt/spectra/internal/database"
	"github.com/nhdewitt/spectra/internal/protocol"
)

// --- Registration ---
// Registration uses token auth (not user or agent auth), only rate limited.

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
	req.RemoteAddr = "10.0.0.1:1234"
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
	req.RemoteAddr = "10.0.0.1:1234"
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
	req.RemoteAddr = "10.0.0.1:1234"
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
	req.RemoteAddr = "10.0.0.1:1234"
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("first use: got %d, want 201", rec.Code)
	}

	// Second use fails
	body, _ = json.Marshal(regReq)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/agent/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "10.0.0.1:1234"
	rec = httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("second use: got %d, want 401", rec.Code)
	}
}

func TestHandleAgentRegister_WrongMethod(t *testing.T) {
	s := New(Config{Port: 8080}, NewMockDB())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agent/register", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", rec.Code)
	}
}

func TestHandleAgentRegister_InvalidJSON(t *testing.T) {
	s := New(Config{Port: 8080}, NewMockDB())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/register", bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "10.0.0.1:1234"
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

// --- Agent Auth Middleware ---

func TestRequireAgentAuth_Success(t *testing.T) {
	s, agentID, secret, _ := newTestServer()

	batch := []RawEnvelope{}
	body, _ := json.Marshal(batch)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/metrics", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "10.0.0.5:1234"
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
	req.RemoteAddr = "10.0.0.5:1234"
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
	req.RemoteAddr = "10.0.0.5:1234"
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
	req.RemoteAddr = "10.0.0.5:1234"
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
	req.RemoteAddr = "10.0.0.5:1234"
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
	req.RemoteAddr = "10.0.0.5:1234"
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
	req.RemoteAddr = "10.0.0.5:1234"
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
	req.RemoteAddr = "10.0.0.5:1234"
	setAgentAuth(req, agentID, secret)
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK && rec.Code != http.StatusNoContent {
		t.Errorf("status: got %d, want 200 or 204", rec.Code)
	}
}

func TestHandleAgentCommand_WithCommand(t *testing.T) {
	s, agentID, secret, _ := newTestServer()
	s.CmdQueue.Send(agentID, protocol.Command{
		ID:   "cmd-123",
		Type: protocol.CmdFetchLogs,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agent/command", nil)
	req.RemoteAddr = "10.0.0.5:1234"
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
	req.RemoteAddr = "10.0.0.5:1234"
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
	req.RemoteAddr = "10.0.0.5:1234"
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
	req.RemoteAddr = "10.0.0.5:1234"
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
	req.RemoteAddr = "10.0.0.5:1234"
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
	req.RemoteAddr = "10.0.0.5:1234"
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
}

// -- SHA-256 ---
func TestRequireAgentAuth_SHA256(t *testing.T) {
	s, _, _, mock := newTestServer()

	// Register agent with SHA-256 secret
	secret := "test-sha256-secret"
	sum := sha256.Sum256([]byte(secret))
	agentID := "550e8400-e29b-41d4-a716-446655440000"
	mock.AgentSHA256[agentID] = sum[:]

	batch := []RawEnvelope{}
	body, _ := json.Marshal(batch)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/metrics", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "10.0.0.5:1234"
	req.Header.Set("X-Agent-ID", agentID)
	req.Header.Set("X-Agent-Secret", secret)
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code == http.StatusUnauthorized {
		t.Error("should not be 401 with valid SHA-256 credentials")
	}
}

func TestRequireAgentAuth_BcryptUpgrade(t *testing.T) {
	s, agentID, secret, mock := newTestServer()
	// newTestServer registers with bcrypt, no SHA-256 yet

	batch := []RawEnvelope{}
	body, _ := json.Marshal(batch)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/metrics", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "10.0.0.5:1234"
	setAgentAuth(req, agentID, secret)
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code == http.StatusUnauthorized {
		t.Error("bcrypt fallback should work")
	}

	// Verify opportunistic upgrade happened
	hash, ok := mock.AgentSHA256[agentID]
	if !ok || len(hash) != sha256.Size {
		t.Error("agent should have been upgraded to SHA-256")
	}

	// Verify the stored hash matches
	expected := sha256.Sum256([]byte(secret))
	if subtle.ConstantTimeCompare(hash, expected[:]) != 1 {
		t.Error("stored SHA-256 hash should match the secret")
	}
}

func TestRequireAgentAuth_SHA256WrongSecret(t *testing.T) {
	s, _, _, mock := newTestServer()

	secret := "correct-secret"
	sum := sha256.Sum256([]byte(secret))
	agentID := "550e8400-e29b-41d4-a716-446655440000"
	mock.AgentSHA256[agentID] = sum[:]

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/metrics", bytes.NewReader([]byte("[]")))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "10.0.0.5:1234"
	req.Header.Set("X-Agent-ID", agentID)
	req.Header.Set("X-Agent-Secret", "wrong-secret")
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
}

// --- Version ---

func TestHandleVersion(t *testing.T) {
	s, _, _, mock := newTestServer()
	_ = mock

	req := httptest.NewRequest(http.MethodGet, "/api/v1/version", nil)
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	for _, key := range []string{"version", "commit", "date"} {
		if _, ok := body[key]; !ok {
			t.Errorf("missing key %q in response", key)
		}
	}
}

// failingWriteDB fails metric writes only.
//
// MockDB.Err fails every query, which means agent auth (GetAgentSecretSHA256)
// and TouchLastSeenIfStale fail too -- the request 401s in middleware, or 500s
// before it reaches the persistence loop, and the test passes for the wrong
// reason. Overriding just the write methods isolates the failure to the thing
// under test.
type failingWriteDB struct {
	*MockDB
	err error
}

func (d *failingWriteDB) InsertCPU(ctx context.Context, p database.InsertCPUParams) error {
	_ = d.MockDB.InsertCPU(ctx, p)
	return d.err
}

func (d *failingWriteDB) InsertMemory(ctx context.Context, p database.InsertMemoryParams) error {
	_ = d.MockDB.InsertMemory(ctx, p)
	return d.err
}

// TestHandleMetrics_PersistFailureReturns500 is the regression test for the
// original bug: the handler wrote 202 and persisted on a detached goroutine, so
// a database failure was invisible to the agent, which had already discarded
// the batch. A 202 has to mean the rows are on disk.
func TestHandleMetrics_PersistFailureReturns500(t *testing.T) {
	s, agentID, secret, mock := newTestServer()
	s.DB = &failingWriteDB{MockDB: mock, err: errors.New("db connection lost")}

	batch := []RawEnvelope{
		{Type: "cpu", Hostname: "test-host", Data: json.RawMessage(`{"usage": 50.0}`)},
	}

	body, _ := json.Marshal(batch)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/metrics", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "198.51.100.7:1234"
	setAgentAuth(req, agentID, secret)
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500 so the agent retains the batch and retries", rec.Code)
	}
}

// TestHandleMetrics_PersistsBeforeResponding pins the ordering. Against the old
// goroutine-based handler the insert had usually not happened by the time the
// response was written, and never deterministically.
func TestHandleMetrics_PersistsBeforeResponding(t *testing.T) {
	s, agentID, secret, mock := newTestServer()

	batch := []RawEnvelope{
		{Type: "cpu", Hostname: "test-host", Data: json.RawMessage(`{"usage": 50.0}`)},
		{Type: "memory", Hostname: "test-host", Data: json.RawMessage(`{"used_pct": 40.0}`)},
	}

	body, _ := json.Marshal(batch)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/metrics", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "198.51.100.7:1234"
	setAgentAuth(req, agentID, secret)
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status: got %d, want 202", rec.Code)
	}
	if mock.InsertCPUCount != 1 {
		t.Errorf("InsertCPU: got %d, want 1 before the response was written", mock.InsertCPUCount)
	}
	if mock.InsertMemoryCount != 1 {
		t.Errorf("InsertMemory: got %d, want 1 before the response was written", mock.InsertMemoryCount)
	}
}

// TestHandleMetrics_StopsAtFirstPersistFailure confirms the handler gives up
// rather than working through a batch against a database that is already
// failing. The agent resends the whole batch regardless.
func TestHandleMetrics_StopsAtFirstPersistFailure(t *testing.T) {
	s, agentID, secret, mock := newTestServer()
	s.DB = &failingWriteDB{MockDB: mock, err: errors.New("db connection lost")}

	batch := []RawEnvelope{
		{Type: "cpu", Hostname: "test-host", Data: json.RawMessage(`{"usage": 50.0}`)},
		{Type: "memory", Hostname: "test-host", Data: json.RawMessage(`{"used_pct": 40.0}`)},
	}

	body, _ := json.Marshal(batch)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/metrics", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "198.51.100.7:1234"
	setAgentAuth(req, agentID, secret)
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d, want 500", rec.Code)
	}
	if mock.InsertMemoryCount != 0 {
		t.Errorf("InsertMemory: got %d, want 0 after the first failure", mock.InsertMemoryCount)
	}
}

// TestHandleMetrics_UndecodableEnvelopeDoesNotFailBatch is the poison-payload
// rule. A malformed or unknown envelope fails identically on every retry, so
// rejecting the batch would wedge that agent's pipeline permanently.
func TestHandleMetrics_UndecodableEnvelopeDoesNotFailBatch(t *testing.T) {
	s, agentID, secret, mock := newTestServer()

	batch := []RawEnvelope{
		{Type: "cpu", Hostname: "test-host", Data: json.RawMessage(`{"usage": "not-a-number"}`)},
		{Type: "not_a_real_metric_type", Hostname: "test-host", Data: json.RawMessage(`{}`)},
		{Type: "memory", Hostname: "test-host", Data: json.RawMessage(`{"used_pct": 40.0}`)},
	}

	body, _ := json.Marshal(batch)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/metrics", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "198.51.100.7:1234"
	setAgentAuth(req, agentID, secret)
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status: got %d, want 202: an undecodable envelope must not fail the batch", rec.Code)
	}
	if mock.InsertCPUCount != 0 {
		t.Errorf("InsertCPU: got %d, want 0 for an envelope that failed to decode", mock.InsertCPUCount)
	}
	if mock.InsertMemoryCount != 1 {
		t.Errorf("InsertMemory: got %d, want 1: the good envelope after it must still persist", mock.InsertMemoryCount)
	}
}
