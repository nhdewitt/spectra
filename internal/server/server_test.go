package server

import (
	"bytes"
	"context"
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
	s := New(cfg)

	if s.Config.Port != 8080 {
		t.Errorf("port: got %d, want 8080", s.Config.Port)
	}
	if s.Store == nil {
		t.Error("store should not be nil")
	}
	if s.Router == nil {
		t.Error("router should not be nil")
	}
}

func TestRoutes_Registered(t *testing.T) {
	s := New(Config{Port: 8080})

	routes := []struct {
		method, path string
	}{
		{"POST", "/api/v1/metrics"},
		{"GET", "/api/v1/agent/command"},
		{"POST", "/api/v1/agent/command_result"},
		{"POST", "/api/v1/agent/register"},
		{"POST", "/admin/trigger_logs"},
		{"POST", "/admin/trigger_disk"},
		{"POST", "/admin/trigger_network"},
	}

	for _, route := range routes {
		t.Run(route.path, func(t *testing.T) {
			req := httptest.NewRequest(route.method, route.path, nil)
			rec := httptest.NewRecorder()

			s.Router.ServeHTTP(rec, req)

			if rec.Code == http.StatusNotFound {
				t.Errorf("route %s %s not registered", route.method, route.path)
			}
		})
	}
}

func TestHandleMetrics_Success(t *testing.T) {
	s := New(Config{Port: 8080})

	batch := []protocol.Envelope{
		{
			Type:     "cpu",
			Hostname: "test-host",
			Data:     protocol.CPUMetric{Usage: 50.0},
		},
	}

	body, _ := json.Marshal(batch)
	req := httptest.NewRequest("POST", "/api/v1/metrics?hostname=test-host", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK && rec.Code != http.StatusAccepted && rec.Code != http.StatusNoContent {
		t.Errorf("status: got %d, want 2xx", rec.Code)
	}
}

func TestHandleMetrics_EmptyBatch(t *testing.T) {
	s := New(Config{Port: 8080})

	body, _ := json.Marshal([]protocol.Envelope{})
	req := httptest.NewRequest("POST", "/api/v1/metrics?hostname=test-host", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	// Should handle empty batch gracefully
	if rec.Code >= 500 {
		t.Errorf("status: got %d, should not be 5xx for empty batch", rec.Code)
	}
}

func TestHandleMetrics_InvalidJSON(t *testing.T) {
	s := New(Config{Port: 8080})

	req := httptest.NewRequest("POST", "/api/v1/metrics", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400 for invalid JSON", rec.Code)
	}
}

func TestHandleAgentRegister_Success(t *testing.T) {
	s := New(Config{Port: 8080})

	info := protocol.HostInfo{
		Hostname: "test-agent",
		OS:       "linux",
		Platform: "ubuntu",
		Arch:     "amd64",
	}

	body, _ := json.Marshal(info)
	req := httptest.NewRequest("POST", "/api/v1/agent/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK && rec.Code != http.StatusCreated {
		t.Errorf("status: got %d, want 200 or 201", rec.Code)
	}

	// Verify agent was stored
	ok := s.Store.Exists("test-agent")
	if !ok {
		t.Error("agent should be registered and accept commands")
	}
}

func TestHandleAgentRegister_InvalidJSON(t *testing.T) {
	s := New(Config{Port: 8080})

	req := httptest.NewRequest("POST", "/api/v1/agent/register", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestHandleAgentCommand_NoCommands(t *testing.T) {
	s := New(Config{Port: 8080})

	s.Store.Register("test-agent")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	req := httptest.NewRequest("GET", "/api/v1/agent/command?hostname=test-agent", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK && rec.Code != http.StatusNoContent {
		t.Errorf("status: got %d, want 200 or 204", rec.Code)
	}
}

func TestHandleAgentCommand_WithCommand(t *testing.T) {
	s := New(Config{Port: 8080})

	s.Store.Register("test-agent")
	s.Store.QueueCommand("test-agent", protocol.Command{
		ID:   "cmd-123",
		Type: protocol.CmdFetchLogs,
	})

	req := httptest.NewRequest("GET", "/api/v1/agent/command?hostname=test-agent", nil)
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

func TestHandleAgentCommand_MissingHostname(t *testing.T) {
	s := New(Config{Port: 8080})

	req := httptest.NewRequest("GET", "/api/v1/agent/command", nil)
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400 for missing hostname", rec.Code)
	}
}

func TestHandleCommandResult_Success(t *testing.T) {
	s := New(Config{Port: 8080})

	log := []protocol.LogEntry{
		{
			Timestamp: time.Now().Unix(),
			Source:    "test-host",
			Level:     protocol.LevelInfo,
			Message:   "Test log message",
		},
	}
	logBytes, _ := json.Marshal(log)

	result := protocol.CommandResult{
		ID:      "cmd-123",
		Type:    protocol.CmdFetchLogs,
		Payload: json.RawMessage(logBytes),
	}

	body, _ := json.Marshal(result)
	req := httptest.NewRequest("POST", "/api/v1/agent/command_result?hostname=test-host", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK && rec.Code != http.StatusAccepted {
		t.Errorf("status: got %d, want 2xx", rec.Code)
	}
}

func TestHandleCommandResult_InvalidJSON(t *testing.T) {
	s := New(Config{Port: 8080})

	req := httptest.NewRequest("POST", "/api/v1/agent/command_result?hostname=test-host", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestHandleAdminTriggerLogs(t *testing.T) {
	s := New(Config{Port: 8080})

	s.Store.Register("test-agent")

	payload := map[string]any{
		"hostname":  "test-agent",
		"min_level": "ERROR",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/admin/trigger_logs?hostname=test-agent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK && rec.Code != http.StatusAccepted {
		t.Errorf("status: got %d, want 2xx", rec.Code)
	}
}

func TestHandleAdminTriggerDisk(t *testing.T) {
	s := New(Config{Port: 8080})

	s.Store.Register("test-agent")

	payload := map[string]any{
		"hostname": "test-agent",
		"path":     "/",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/admin/trigger_disk?hostname=test-agent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK && rec.Code != http.StatusAccepted {
		t.Errorf("status: got %d, want 2xx", rec.Code)
	}
}

func TestHandleAdminTriggerNetwork(t *testing.T) {
	s := New(Config{Port: 8080})

	s.Store.Register("test-agent")

	payload := map[string]any{
		"hostname": "test-agent",
		"action":   "ping",
		"target":   "8.8.8.8",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/admin/trigger_network?hostname=test-agent&action=ping&target=8.8.8.8", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK && rec.Code != http.StatusAccepted {
		t.Errorf("status: got %d, want 2xx", rec.Code)
	}
}

func TestHandleAdminTrigger_UnknownAgent(t *testing.T) {
	s := New(Config{Port: 8080})

	payload := map[string]any{
		"hostname": "nonexistent-agent",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/admin/trigger_logs?hostname=nonexistent-agent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound && rec.Code != http.StatusBadRequest {
		t.Logf("status for unknown agent: %d", rec.Code)
	}
}

func TestConfig_Defaults(t *testing.T) {
	cfg := Config{}
	if cfg.Port != 0 {
		t.Errorf("default port should be 0, got %d", cfg.Port)
	}
}

func BenchmarkHandleMetrics_SingleEnvelope(b *testing.B) {
	s := New(Config{Port: 8080})

	batch := []protocol.Envelope{
		{
			Type:     "cpu",
			Hostname: "test-host",
			Data:     protocol.CPUMetric{Usage: 50.0},
		},
	}
	body, _ := json.Marshal(batch)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		req := httptest.NewRequest("POST", "/api/v1/metrics", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		s.Router.ServeHTTP(rec, req)
	}
}

func BenchmarkHandleMetrics_LargeBatch(b *testing.B) {
	s := New(Config{Port: 8080})

	batch := make([]protocol.Envelope, 50)
	for i := range batch {
		batch[i] = protocol.Envelope{
			Type:     "cpu",
			Hostname: "test-host",
			Data:     protocol.CPUMetric{Usage: float64(i)},
		}
	}
	body, _ := json.Marshal(batch)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		req := httptest.NewRequest("POST", "/api/v1/metrics", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		s.Router.ServeHTTP(rec, req)
	}
}

func BenchmarkHandleAgentRegister(b *testing.B) {
	s := New(Config{Port: 8080})

	info := protocol.HostInfo{
		Hostname: "bench-agent",
		OS:       "linux",
		Platform: "ubuntu",
		Arch:     "amd64",
		CPUModel: "Intel i7",
		CPUCores: 8,
		RAMTotal: 16000000000,
	}
	body, _ := json.Marshal(info)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		req := httptest.NewRequest("POST", "/api/v1/agent/register", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		s.Router.ServeHTTP(rec, req)
	}
}

func BenchmarkHandleAgentCommand_NoCommand(b *testing.B) {
	s := New(Config{Port: 8080, CommandTimeout: 1 * time.Millisecond})
	s.Store.Register("bench-agent")

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		req := httptest.NewRequest("GET", "/api/v1/agent/command?hostname=bench-agent", nil)
		rec := httptest.NewRecorder()
		s.Router.ServeHTTP(rec, req)
	}
}

func BenchmarkHandleAgentCommand_WithCommand(b *testing.B) {
	s := New(Config{Port: 8080, CommandTimeout: 1 * time.Second})
	s.Store.Register("bench-agent")

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		s.Store.QueueCommand("bench-agent", protocol.Command{
			ID:   "cmd-123",
			Type: protocol.CmdFetchLogs,
		})

		req := httptest.NewRequest("GET", "/api/v1/agent/command?hostname=bench-agent", nil)
		rec := httptest.NewRecorder()
		s.Router.ServeHTTP(rec, req)

		io.Copy(io.Discard, rec.Body)
	}
}

func BenchmarkHandleCommandResult(b *testing.B) {
	s := New(Config{Port: 8080, CommandTimeout: 1 * time.Second})

	result := protocol.CommandResult{
		ID:      "cmd-123",
		Type:    protocol.CmdFetchLogs,
		Payload: json.RawMessage(`{"logs":[{"timestamp":1234567890,"source":"test","level":"ERROR","message":"test message"}]}`),
	}
	body, _ := json.Marshal(result)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		req := httptest.NewRequest("POST", "/api/v1/agent/command_result", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		s.Router.ServeHTTP(rec, req)
	}
}
