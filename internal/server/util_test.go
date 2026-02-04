package server

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func TestDecodeJSONBody_Success(t *testing.T) {
	data := map[string]string{"key": "value"}
	body, _ := json.Marshal(data)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	var target map[string]string
	err := decodeJSONBody(req, &target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if target["key"] != "value" {
		t.Errorf("got %s, want value", target["key"])
	}
}

func TestDecodeJSONBody_Gzip(t *testing.T) {
	data := map[string]string{"key": "compressed"}
	jsonData, _ := json.Marshal(data)

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	gz.Write(jsonData)
	gz.Close()

	req := httptest.NewRequest(http.MethodPost, "/", &buf)
	req.Header.Set("Content-Encoding", "gzip")

	var target map[string]string
	err := decodeJSONBody(req, &target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if target["key"] != "compressed" {
		t.Errorf("got %s, want compressed", target["key"])
	}
}

func TestDecodeJSONBody_InvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("not json")))

	var target map[string]string
	err := decodeJSONBody(req, &target)

	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestDecodeJSONBody_InvalidGzip(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("not gzip data")))
	req.Header.Set("Content-Encoding", "gzip")

	var target map[string]string
	err := decodeJSONBody(req, &target)

	if err == nil {
		t.Error("expected error for invalid gzip")
	}
}

func TestDecodeJSONBody_EmptyBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte{}))

	var target map[string]string
	err := decodeJSONBody(req, &target)

	if err == nil {
		t.Error("expected error for empty body")
	}
}

func TestDecodeJSONBody_Struct(t *testing.T) {
	info := protocol.HostInfo{
		Hostname: "test-host",
		OS:       "linux",
		CPUCores: 8,
	}
	body, _ := json.Marshal(info)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))

	var target protocol.HostInfo
	err := decodeJSONBody(req, &target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if target.Hostname != "test-host" {
		t.Errorf("Hostname = %s, want test-host", target.Hostname)
	}
	if target.CPUCores != 8 {
		t.Errorf("CPUCores = %d, want 8", target.CPUCores)
	}
}

func TestGetHostname_Success(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?hostname=test-agent", nil)
	rec := httptest.NewRecorder()

	hostname, ok := getHostname(rec, req)

	if !ok {
		t.Error("expected ok to be true")
	}
	if hostname != "test-agent" {
		t.Errorf("hostname = %s, want test-agent", hostname)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestGetHostname_Missing(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	hostname, ok := getHostname(rec, req)

	if ok {
		t.Error("expected ok to be false")
	}
	if hostname != "" {
		t.Errorf("hostname should be empty, got %s", hostname)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestGetHostname_Empty(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?hostname=", nil)
	rec := httptest.NewRecorder()

	hostname, ok := getHostname(rec, req)

	if ok {
		t.Error("expected ok to be false for empty hostname")
	}
	if hostname != "" {
		t.Errorf("hostname should be empty, got %s", hostname)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestRespondJSON_Success(t *testing.T) {
	rec := httptest.NewRecorder()

	data := map[string]string{"status": "ok"}
	respondJSON(rec, http.StatusOK, data)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %s, want application/json", rec.Header().Get("Content-Type"))
	}

	var response map[string]string
	json.NewDecoder(rec.Body).Decode(&response)
	if response["status"] != "ok" {
		t.Errorf("response status = %s, want ok", response["status"])
	}
}

func TestRespondJSON_NilData(t *testing.T) {
	rec := httptest.NewRecorder()

	respondJSON(rec, http.StatusNoContent, nil)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %s, want application/json", rec.Header().Get("Content-Type"))
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body should be empty, got %s", rec.Body.String())
	}
}

func TestRespondJSON_Struct(t *testing.T) {
	rec := httptest.NewRecorder()

	cmd := protocol.Command{ID: "cmd-123", Type: protocol.CmdFetchLogs}
	respondJSON(rec, http.StatusOK, cmd)

	var response protocol.Command
	json.NewDecoder(rec.Body).Decode(&response)
	if response.ID != "cmd-123" {
		t.Errorf("response ID = %s, want cmd-123", response.ID)
	}
}

func TestQueueHelper_Success(t *testing.T) {
	s := New(Config{Port: 8080})
	s.Store.Register("test-agent")

	rec := httptest.NewRecorder()
	s.queueHelper(rec, "test-agent", protocol.CmdFetchLogs, []byte(`{}`), "Queued!")

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != "Queued!\n" {
		t.Errorf("body = %q, want 'Queued!\\n'", rec.Body.String())
	}
}

func TestQueueHelper_UnregisteredAgent(t *testing.T) {
	s := New(Config{Port: 8080})

	rec := httptest.NewRecorder()
	s.queueHelper(rec, "unknown-agent", protocol.CmdFetchLogs, []byte(`{}`), "Queued!")

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
}

func TestQueueHelper_QueueFull(t *testing.T) {
	s := New(Config{Port: 8080})
	s.Store.Register("test-agent")

	for range 10 {
		s.Store.QueueCommand("test-agent", protocol.Command{ID: "cmd"})
	}

	rec := httptest.NewRecorder()
	s.queueHelper(rec, "test-agent", protocol.CmdFetchLogs, []byte(`{}`), "Queued!")

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input    uint64
		expected string
	}{
		{0, "0 B"},
		{1, "1 B"},
		{500, "500 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1572864, "1.5 MB"},
		{1073741824, "1.0 GB"},
		{1610612736, "1.5 GB"},
		{1099511627776, "1.0 TB"},
		{1649267441664, "1.5 TB"},
		{1125899906842624, "1.0 PB"},
		{1152921504606846976, "1.0 EB"},
		{16000000000, "14.9 GB"},
		{500000000000, "465.7 GB"},
		{22105, "21.6 KB"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := formatBytes(tt.input)
			if got != tt.expected {
				t.Errorf("formatBytes(%d) = %s, want %s", tt.input, got, tt.expected)
			}
		})
	}
}

func BenchmarkDecodeJSONBody_Small(b *testing.B) {
	data := map[string]string{"key": "value"}
	body, _ := json.Marshal(data)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
		var target map[string]string
		decodeJSONBody(req, &target)
	}
}

func BenchmarkDecodeJSONBody_Large(b *testing.B) {
	procs := make([]protocol.ProcessMetric, 200)
	for i := range procs {
		procs[i] = protocol.ProcessMetric{Pid: i, Name: "process", CPUPercent: 1.0}
	}
	body, _ := json.Marshal(protocol.ProcessListMetric{Processes: procs})
	b.Logf("Payload size: %d bytes", len(body))

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
		var target protocol.ProcessListMetric
		decodeJSONBody(req, &target)
	}
}

func BenchmarkDecodeJSONBody_Gzip(b *testing.B) {
	data := map[string]string{"key": "value"}
	jsonData, _ := json.Marshal(data)

	var compressed bytes.Buffer
	gz := gzip.NewWriter(&compressed)
	gz.Write(jsonData)
	gz.Close()
	gzipData := compressed.Bytes()

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(gzipData))
		req.Header.Set("Content-Encoding", "gzip")
		var target map[string]string
		decodeJSONBody(req, &target)
	}
}

func BenchmarkGetHostname(b *testing.B) {
	req := httptest.NewRequest(http.MethodGet, "/?hostname=test-agent", nil)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		rec := httptest.NewRecorder()
		getHostname(rec, req)
	}
}

func BenchmarkRespondJSON_Small(b *testing.B) {
	data := map[string]string{"status": "ok"}

	b.ReportAllocs()
	for b.Loop() {
		rec := httptest.NewRecorder()
		respondJSON(rec, http.StatusOK, data)
	}
}

func BenchmarkRespondJSON_Command(b *testing.B) {
	cmd := protocol.Command{
		ID:      "cmd-123",
		Type:    protocol.CmdFetchLogs,
		Payload: []byte(`{"min_level":"ERROR"}`),
	}

	b.ReportAllocs()
	for b.Loop() {
		rec := httptest.NewRecorder()
		respondJSON(rec, http.StatusOK, cmd)
	}
}

func BenchmarkFormatBytes(b *testing.B) {
	values := []uint64{0, 1024, 1048576, 1073741824, 16000000000}

	b.ReportAllocs()
	for b.Loop() {
		for _, v := range values {
			formatBytes(v)
		}
	}
}

func BenchmarkQueueHelper(b *testing.B) {
	s := New(Config{Port: 8080})
	s.Store.Register("bench-agent")

	payload := []byte(`{"min_level":"ERROR"}`)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		b.StopTimer()
		for {
			cmd, _ := s.Store.WaitForCommand(context.TODO(), "bench-agent", 0)
			if cmd.ID == "" {
				break
			}
		}
		b.StartTimer()

		rec := httptest.NewRecorder()
		s.queueHelper(rec, "bench-agent", protocol.CmdFetchLogs, payload, "Queued!")
	}
}
