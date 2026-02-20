package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func TestNew(t *testing.T) {
	cfg := Config{Port: 8080}
	s := New(cfg, NewMockDB())

	if s.Config.Port != 8080 {
		t.Errorf("port: got %d, want 8080", s.Config.Port)
	}
	if s.Store == nil {
		t.Error("store should not be nil")
	}
	if s.Tokens == nil {
		t.Error("tokens should not be nil")
	}
	if s.Router == nil {
		t.Error("router should not be nil")
	}
	if s.DB == nil {
		t.Error("DB should not be nil")
	}
}

func TestNew_DefaultCommandTimeout(t *testing.T) {
	s := New(Config{Port: 8080}, NewMockDB())

	if s.Config.CommandTimeout != 30*time.Second {
		t.Errorf("CommandTimeout: got %v, want 30s", s.Config.CommandTimeout)
	}
}

func TestNew_CustomCommandTimeout(t *testing.T) {
	s := New(Config{Port: 8080, CommandTimeout: 5 * time.Second}, NewMockDB())

	if s.Config.CommandTimeout != 5*time.Second {
		t.Errorf("CommandTimeout: got %v, want 5s", s.Config.CommandTimeout)
	}
}

func TestRoutes_Registered(t *testing.T) {
	s, agentID, secret, _ := newTestServer()

	tests := []struct {
		name       string
		method     string
		path       string
		auth       bool
		wantNotNot int // should NOT be this status (404 = route not registered)
	}{
		{"register", "POST", "/api/v1/agent/register", false, 404},
		{"metrics", "POST", "/api/v1/agent/metrics", true, 404},
		{"command", "GET", "/api/v1/agent/command", true, 404},
		{"command result", "POST", "/api/v1/agent/command/result", true, 404},
		{"admin logs", "POST", "/api/v1/admin/logs", false, 404},
		{"admin disk", "POST", "/api/v1/admin/disk", false, 404},
		{"admin network", "POST", "/api/v1/admin/network", false, 404},
		{"admin tokens", "POST", "/api/v1/admin/tokens", false, 404},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, bytes.NewReader([]byte("{}")))
			if tt.auth {
				setAgentAuth(req, agentID, secret)
			}
			rec := httptest.NewRecorder()

			s.Router.ServeHTTP(rec, req)

			if rec.Code == tt.wantNotNot {
				t.Errorf("route %s %s returned %d (not registered)", tt.method, tt.path, rec.Code)
			}
		})
	}
}

func TestRoutes_AuthRequired(t *testing.T) {
	s, _, _, _ := newTestServer()

	authedRoutes := []struct {
		method, path string
	}{
		{"POST", "/api/v1/agent/metrics"},
		{"GET", "/api/v1/agent/command"},
		{"POST", "/api/v1/agent/command/result"},
	}

	for _, route := range authedRoutes {
		t.Run(route.path, func(t *testing.T) {
			req := httptest.NewRequest(route.method, route.path, bytes.NewReader([]byte("{}")))
			rec := httptest.NewRecorder()

			s.Router.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("%s %s without auth: got %d, want 401", route.method, route.path, rec.Code)
			}
		})
	}
}

func TestConfig_Defaults(t *testing.T) {
	cfg := Config{}
	if cfg.Port != 0 {
		t.Errorf("default port should be 0, got %d", cfg.Port)
	}
}

// --- Benchmarks ---

func BenchmarkHandleMetrics_SingleEnvelope(b *testing.B) {
	s, agentID, secret, _ := newTestServer()

	batch := []RawEnvelope{
		{
			Type:     "cpu",
			Hostname: "test-host",
			Data:     json.RawMessage(`{"usage": 50.0}`),
		},
	}
	body, _ := json.Marshal(batch)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		req := httptest.NewRequest("POST", "/api/v1/agent/metrics", bytes.NewReader(body))
		setAgentAuth(req, agentID, secret)
		rec := httptest.NewRecorder()
		s.Router.ServeHTTP(rec, req)
	}
}

func BenchmarkHandleMetrics_LargeBatch(b *testing.B) {
	s, agentID, secret, _ := newTestServer()

	batch := make([]RawEnvelope, 50)
	for i := range batch {
		data, _ := json.Marshal(protocol.CPUMetric{Usage: float64(i)})
		batch[i] = RawEnvelope{
			Type:     "cpu",
			Hostname: "test-host",
			Data:     json.RawMessage(data),
		}
	}
	body, _ := json.Marshal(batch)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		req := httptest.NewRequest("POST", "/api/v1/agent/metrics", bytes.NewReader(body))
		setAgentAuth(req, agentID, secret)
		rec := httptest.NewRecorder()
		s.Router.ServeHTTP(rec, req)
	}
}

func BenchmarkHandleAgentRegister(b *testing.B) {
	s := New(Config{Port: 8080}, NewMockDB())

	info := protocol.HostInfo{
		Hostname: "bench-agent",
		OS:       "linux",
		Platform: "ubuntu",
		Arch:     "amd64",
		CPUModel: "Intel i7",
		CPUCores: 8,
		RAMTotal: 16000000000,
	}

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		token := s.Tokens.Generate(24 * time.Hour)
		regReq := protocol.RegisterRequest{Token: token, Info: info}
		body, _ := json.Marshal(regReq)

		req := httptest.NewRequest("POST", "/api/v1/agent/register", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		s.Router.ServeHTTP(rec, req)
	}
}

func BenchmarkHandleAgentCommand_NoCommand(b *testing.B) {
	s, agentID, secret, _ := newTestServer()
	s.Config.CommandTimeout = 1 * time.Millisecond

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		req := httptest.NewRequest("GET", "/api/v1/agent/command", nil)
		setAgentAuth(req, agentID, secret)
		rec := httptest.NewRecorder()
		s.Router.ServeHTTP(rec, req)
	}
}

func BenchmarkHandleAgentCommand_WithCommand(b *testing.B) {
	s, agentID, secret, _ := newTestServer()
	s.Config.CommandTimeout = 1 * time.Second

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		s.Store.QueueCommand(agentID, protocol.Command{
			ID:   "cmd-123",
			Type: protocol.CmdFetchLogs,
		})

		req := httptest.NewRequest("GET", "/api/v1/agent/command", nil)
		setAgentAuth(req, agentID, secret)
		rec := httptest.NewRecorder()
		s.Router.ServeHTTP(rec, req)

		io.Copy(io.Discard, rec.Body)
	}
}

func BenchmarkHandleCommandResult(b *testing.B) {
	s, agentID, secret, _ := newTestServer()

	result := protocol.CommandResult{
		ID:      "cmd-123",
		Type:    protocol.CmdFetchLogs,
		Payload: json.RawMessage(`{"logs":[{"timestamp":1234567890,"source":"test","level":"ERROR","message":"test message"}]}`),
	}
	body, _ := json.Marshal(result)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		req := httptest.NewRequest("POST", "/api/v1/agent/command/result", bytes.NewReader(body))
		setAgentAuth(req, agentID, secret)
		rec := httptest.NewRecorder()
		s.Router.ServeHTTP(rec, req)
	}
}

func BenchmarkRequireAgentAuth(b *testing.B) {
	s, agentID, secret, _ := newTestServer()

	body := []byte("[]")

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		req := httptest.NewRequest("POST", "/api/v1/agent/metrics", bytes.NewReader(body))
		setAgentAuth(req, agentID, secret)
		rec := httptest.NewRecorder()
		s.Router.ServeHTTP(rec, req)
	}
}
