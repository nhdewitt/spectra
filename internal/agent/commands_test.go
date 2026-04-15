package agent

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func TestPollOnce_NoCommand(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	a := newTestAgentWithLogger()
	a.Config.BaseURL = srv.URL
	a.Config.CommandPath = "/api/v1/agent/command"

	a.pollOnce(context.Background(), srv.URL+"/api/v1/agent/command")
}

func TestPollOnce_ReceivesCommand(t *testing.T) {
	cmd := protocol.Command{
		ID:   "cmd-123",
		Type: protocol.CmdFetchLogs,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cmd)
	}))
	defer srv.Close()

	a := newTestAgentWithLogger()
	a.Config.BaseURL = srv.URL
	a.Config.CommandPath = "/api/v1/agent/command"

	// pollOnce spawns a goroutine for handleCommand — just verify no panic
	a.pollOnce(context.Background(), srv.URL+"/api/v1/agent/command")

	// Give the goroutine time to start
	time.Sleep(100 * time.Millisecond)
}

func TestPollOnce_ServerDown(t *testing.T) {
	a := newTestAgentWithLogger()

	// Should not panic
	a.pollOnce(context.Background(), "http://127.0.0.1:1/api/v1/agent/command")
}

func TestPollOnce_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	a := newTestAgentWithLogger()

	// Should not panic — non-200 is silently ignored
	a.pollOnce(context.Background(), srv.URL+"/api/v1/agent/command")
}

func TestPollOnce_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	a := newTestAgentWithLogger()

	// Should not panic — decode error is silently ignored
	a.pollOnce(context.Background(), srv.URL+"/api/v1/agent/command")
}

func TestPollOnce_SetsAuthHeaders(t *testing.T) {
	var agentID, agentSecret string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agentID = r.Header.Get("X-Agent-ID")
		agentSecret = r.Header.Get("X-Agent-Secret")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	a := newTestAgentWithLogger()

	a.pollOnce(context.Background(), srv.URL+"/api/v1/agent/command")

	if agentID != a.Identity.ID {
		t.Errorf("expected X-Agent-ID %q, got %q", a.Identity.ID, agentID)
	}
	if agentSecret != a.Identity.Secret {
		t.Errorf("expected X-Agent-Secret %q, got %q", a.Identity.Secret, agentSecret)
	}
}

func TestPollOnce_NoContentEncoding(t *testing.T) {
	var contentEncoding string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentEncoding = r.Header.Get("Content-Encoding")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	a := newTestAgentWithLogger()

	a.pollOnce(context.Background(), srv.URL+"/api/v1/agent/command")

	if contentEncoding != "" {
		t.Errorf("expected no Content-Encoding, got %q", contentEncoding)
	}
}

func TestUploadCommandResult_Success(t *testing.T) {
	var received protocol.CommandResult
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gz, _ := gzip.NewReader(r.Body)
		json.NewDecoder(gz).Decode(&received)
		gz.Close()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := newTestAgentWithLogger()
	a.Config.BaseURL = srv.URL

	cmd := protocol.Command{ID: "cmd-456", Type: protocol.CmdListMounts}
	data := map[string]string{"mount": "/"}

	err := a.uploadCommandResult(context.Background(), cmd, data, nil)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if received.ID != "cmd-456" {
		t.Errorf("expected command ID 'cmd-456', got %q", received.ID)
	}
	if received.Type != protocol.CmdListMounts {
		t.Errorf("expected type CmdListMounts, got %q", received.Type)
	}
	if received.Error != "" {
		t.Errorf("expected no error in result, got %q", received.Error)
	}
	if received.Payload == nil {
		t.Error("expected non-nil payload")
	}
}

func TestUploadCommandResult_WithError(t *testing.T) {
	var received protocol.CommandResult
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gz, _ := gzip.NewReader(r.Body)
		json.NewDecoder(gz).Decode(&received)
		gz.Close()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := newTestAgentWithLogger()
	a.Config.BaseURL = srv.URL

	cmd := protocol.Command{ID: "cmd-789", Type: protocol.CmdDiskUsage}
	cmdErr := io.ErrUnexpectedEOF

	err := a.uploadCommandResult(context.Background(), cmd, nil, cmdErr)
	if err != nil {
		t.Fatalf("expected no error from upload, got: %v", err)
	}

	if received.Error == "" {
		t.Error("expected error string in result")
	}
}

func TestUploadCommandResult_NilData(t *testing.T) {
	var received protocol.CommandResult
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gz, _ := gzip.NewReader(r.Body)
		json.NewDecoder(gz).Decode(&received)
		gz.Close()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := newTestAgentWithLogger()
	a.Config.BaseURL = srv.URL

	cmd := protocol.Command{ID: "cmd-nil", Type: protocol.CmdNetworkDiag}
	err := a.uploadCommandResult(context.Background(), cmd, nil, nil)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if received.Error != "" {
		t.Errorf("expected no error string, got %q", received.Error)
	}
	if len(received.Payload) > 0 && string(received.Payload) != "null" {
		t.Errorf("expected nil/empty payload, got %s", received.Payload)
	}
}

func TestUploadCommandResult_ServerRejects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("bad result"))
	}))
	defer srv.Close()

	a := newTestAgentWithLogger()
	a.Config.BaseURL = srv.URL

	cmd := protocol.Command{ID: "cmd-rej", Type: protocol.CmdFetchLogs}
	err := a.uploadCommandResult(context.Background(), cmd, "data", nil)

	if err == nil {
		t.Fatal("expected error for rejected result")
	}
}

func TestUploadCommandResult_ServerDown(t *testing.T) {
	a := newTestAgentWithLogger()
	a.Config.BaseURL = "http://127.0.0.1:1"

	cmd := protocol.Command{ID: "cmd-down", Type: protocol.CmdFetchLogs}
	err := a.uploadCommandResult(context.Background(), cmd, "data", nil)

	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

func TestUploadCommandResult_GzipCompressed(t *testing.T) {
	var contentEncoding string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentEncoding = r.Header.Get("Content-Encoding")
		io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := newTestAgentWithLogger()
	a.Config.BaseURL = srv.URL

	cmd := protocol.Command{ID: "cmd-gz", Type: protocol.CmdFetchLogs}
	a.uploadCommandResult(context.Background(), cmd, "data", nil)

	if contentEncoding != "gzip" {
		t.Errorf("expected Content-Encoding gzip, got %q", contentEncoding)
	}
}

func TestHandleCommand_UnknownType(t *testing.T) {
	var received protocol.CommandResult
	var callCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		gz, _ := gzip.NewReader(r.Body)
		json.NewDecoder(gz).Decode(&received)
		gz.Close()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := newTestAgentWithLogger()
	a.Config.BaseURL = srv.URL

	cmd := protocol.Command{ID: "cmd-unk", Type: "UNKNOWN_CMD"}
	a.handleCommand(context.Background(), cmd)

	if callCount.Load() != 1 {
		t.Fatalf("expected 1 upload call, got %d", callCount.Load())
	}
	if received.Error == "" {
		t.Error("expected error for unknown command type")
	}
}

func TestHandleCommand_ListMounts(t *testing.T) {
	var received protocol.CommandResult
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gz, _ := gzip.NewReader(r.Body)
		json.NewDecoder(gz).Decode(&received)
		gz.Close()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := newTestAgentWithLogger()
	a.Config.BaseURL = srv.URL

	cmd := protocol.Command{ID: "cmd-mounts", Type: protocol.CmdListMounts}
	a.handleCommand(context.Background(), cmd)

	if received.ID != "cmd-mounts" {
		t.Errorf("expected ID 'cmd-mounts', got %q", received.ID)
	}
	// ListMounts should not produce an error
	if received.Error != "" {
		t.Errorf("expected no error for ListMounts, got %q", received.Error)
	}
}

func TestHandleCommand_ContextTimeout(t *testing.T) {
	var received protocol.CommandResult
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gz, _ := gzip.NewReader(r.Body)
		json.NewDecoder(gz).Decode(&received)
		gz.Close()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := newTestAgentWithLogger()
	a.Config.BaseURL = srv.URL

	// Parent context already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cmd := protocol.Command{ID: "cmd-timeout", Type: protocol.CmdListMounts}
	a.handleCommand(ctx, cmd)

	// Should still attempt to upload the result
}
