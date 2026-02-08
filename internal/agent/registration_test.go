package agent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestRegister_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/agent/register" {
			t.Errorf("path: got %s, want /api/v1/agent/register", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method: got %s, want POST", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type: got %s, want application/json", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("Content-Encoding") != "" {
			t.Errorf("Content-Encoding should be empty, got %s", r.Header.Get("Content-Encoding"))
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	a := New(Config{
		BaseURL:  server.URL,
		Hostname: "test-agent",
	})

	err := a.Register()
	if err != nil {
		t.Errorf("Register failed: %v", err)
	}
}

func TestRegister_SuccessCreated(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	a := New(Config{
		BaseURL:  server.URL,
		Hostname: "test-agent",
	})

	err := a.Register()
	if err != nil {
		t.Errorf("Register should accept 201 Created: %v", err)
	}
}

func TestRegister_ServerError(t *testing.T) {
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	a := New(Config{
		BaseURL:  server.URL,
		Hostname: "test-agent",
	})
	a.RetryConfig = RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     10 * time.Millisecond,
		Multiplier:   2.0,
	}

	err := a.Register()
	if err == nil {
		t.Error("expected error for server error")
	}
	// Should have retried 3 times
	if atomic.LoadInt32(&requestCount) != 3 {
		t.Errorf("request count: got %d, want 3", requestCount)
	}
}

func TestRetryConfig_Delay(t *testing.T) {
	rc := RetryConfig{
		InitialDelay: 1 * time.Second,
		MaxDelay:     10 * time.Second,
		Multiplier:   2.0,
	}

	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{0, 1 * time.Second},
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{3, 8 * time.Second},
		{4, 10 * time.Second},
		{5, 10 * time.Second},
	}

	for _, tt := range tests {
		got := rc.Delay(tt.attempt)
		if got != tt.want {
			t.Errorf("Delay(%d) = %v, want %v", tt.attempt, got, tt.want)
		}
	}
}

func TestRegister_RetrySuccess(t *testing.T) {
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)
		if count < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	a := New(Config{
		BaseURL:  server.URL,
		Hostname: "test-agent",
	})
	a.RetryConfig = RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     10 * time.Millisecond,
		Multiplier:   2.0,
	}

	err := a.Register()
	if err != nil {
		t.Errorf("Register should succeed after retries: %v", err)
	}
	if atomic.LoadInt32(&requestCount) != 3 {
		t.Errorf("request count: got %d, want 3", requestCount)
	}
}

func TestRegister_ConnectionError(t *testing.T) {
	a := New(Config{
		BaseURL:  "http://localhost:59999",
		Hostname: "test-agent",
	})
	a.RetryConfig = RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     10 * time.Millisecond,
		Multiplier:   2.0,
	}

	err := a.Register()
	if err == nil {
		t.Error("expected error for connection failure")
	}
}

func TestRegister_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	a := New(Config{
		BaseURL:  server.URL,
		Hostname: "test-agent",
	})
	a.RetryConfig = RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     10 * time.Millisecond,
		Multiplier:   2.0,
	}
	a.cancel()

	err := a.Register()
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

func TestRegister_HostnameInPayload(t *testing.T) {
	var receivedHostname string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("failed to decode payload: %v", err)
		}
		if hostname, ok := payload["hostname"].(string); ok {
			receivedHostname = hostname
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	a := New(Config{
		BaseURL:  server.URL,
		Hostname: "test-agent",
	})

	err := a.Register()
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if receivedHostname != "test-agent" {
		t.Errorf("hostname: got %s, want test-agent", receivedHostname)
	}
}

func TestRegister_BadRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	a := New(Config{
		BaseURL:  server.URL,
		Hostname: "test-agent",
	})

	err := a.Register()
	if err == nil {
		t.Error("expected error for 400 response")
	}
}

func TestRegister_UserAgent(t *testing.T) {
	var receivedUA string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	a := New(Config{
		BaseURL:  server.URL,
		Hostname: "test-agent",
	})

	a.Register()
	if receivedUA != "Spectra-Agent/1.0" {
		t.Errorf("User-Agent: got %s, want Spectra-Agent/1.0", receivedUA)
	}
}

func BenchmarkRegister(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	a := New(Config{
		BaseURL:  server.URL,
		Hostname: "bench-agent",
	})

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		a.Register()
	}
}
