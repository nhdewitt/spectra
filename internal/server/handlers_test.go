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

func TestHandleAgentRegister_MethodNotAllowed(t *testing.T) {
	s := New(Config{Port: 8080})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agent/register", nil)
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status: got %d, want 405", rec.Code)
	}
}

func TestHandleMetrics_MissingHostname(t *testing.T) {
	s := New(Config{Port: 8080})

	body, _ := json.Marshal([]RawEnvelope{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/metrics", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestHandleCommandResult_MissingHostname(t *testing.T) {
	s := New(Config{Port: 8080})

	result := protocol.CommandResult{ID: "cmd-123", Type: protocol.CmdFetchLogs}
	body, _ := json.Marshal(result)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/command_result", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestHandleCommandResult_WithError(t *testing.T) {
	s := New(Config{Port: 8080})

	result := protocol.CommandResult{
		ID:    "cmd-123",
		Type:  protocol.CmdFetchLogs,
		Error: "permission denied",
	}

	body, _ := json.Marshal(result)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/command_result?hostname=test-agent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestHandleAdminTriggerLogs_MissingHostname(t *testing.T) {
	s := New(Config{Port: 8080})

	req := httptest.NewRequest(http.MethodPost, "/admin/trigger_logs", nil)
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestHandleAdminTriggerDisk_MissingHostname(t *testing.T) {
	s := New(Config{Port: 8080})

	req := httptest.NewRequest(http.MethodPost, "/admin/trigger_disk", nil)
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestHandleAdminTriggerNetwork_MissingHostname(t *testing.T) {
	s := New(Config{Port: 8080})

	req := httptest.NewRequest(http.MethodPost, "/admin/trigger_network?action=ping", nil)
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestHandleAdminTriggerNetwork_MissingAction(t *testing.T) {
	s := New(Config{Port: 8080})
	s.Store.Register("test-agent")

	req := httptest.NewRequest(http.MethodPost, "/admin/trigger_network?hostname=test-agent", nil)
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestHandleAgentCommand_UnregisteredAgent(t *testing.T) {
	s := New(Config{Port: 8080, CommandTimeout: 1 * time.Millisecond})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agent/command?hostname=unknown-agent", nil)
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status: got %d, want 204", rec.Code)
	}
}
