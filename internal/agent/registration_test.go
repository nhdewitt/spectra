package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

// fakeRegisterServer returns an httptest.Server that mimics successful registration.
func fakeRegisterServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(protocol.RegisterResponse{
			AgentID: "test-agent-id",
			Secret:  "test-agent-secret",
		})
	}))
}

func testConfig(t *testing.T, serverURL string) Config {
	return Config{
		BaseURL:           serverURL,
		Hostname:          "test-agent",
		RegistrationToken: "tok-123",
		IdentityPath:      filepath.Join(t.TempDir(), "agent-id.json"),
	}
}

func benchConfig(b *testing.B, serverURL string) Config {
	return Config{
		BaseURL:           serverURL,
		Hostname:          "bench-agent",
		RegistrationToken: "bench-123",
		IdentityPath:      filepath.Join(b.TempDir(), "agent-id.json"),
	}
}

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

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(protocol.RegisterResponse{
			AgentID: "test-id",
			Secret:  "test-secret",
		})
	}))
	defer server.Close()

	a := New(testConfig(t, server.URL))

	err := a.Register(context.Background())
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if a.Identity.ID != "test-id" {
		t.Errorf("Identity.ID: got %s, want test-id", a.Identity.ID)
	}
	if a.Identity.Secret != "test-secret" {
		t.Errorf("Identity.Secret: got %s, want test-secret", a.Identity.Secret)
	}
}

func TestRegister_SuccessCreated(t *testing.T) {
	server := fakeRegisterServer(t)
	defer server.Close()

	a := New(testConfig(t, server.URL))

	err := a.Register(context.Background())
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

	a := New(testConfig(t, server.URL))
	a.RetryConfig = RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     10 * time.Millisecond,
		Multiplier:   2.0,
	}

	err := a.Register(context.Background())
	if err == nil {
		t.Error("expected error for server error")
	}
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
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(protocol.RegisterResponse{
			AgentID: "retry-id",
			Secret:  "retry-secret",
		})
	}))
	defer server.Close()

	a := New(testConfig(t, server.URL))
	a.RetryConfig = RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     10 * time.Millisecond,
		Multiplier:   2.0,
	}

	err := a.Register(context.Background())
	if err != nil {
		t.Fatalf("Register should succeed after retries: %v", err)
	}
	if atomic.LoadInt32(&requestCount) != 3 {
		t.Errorf("request count: got %d, want 3", requestCount)
	}
	if a.Identity.ID != "retry-id" {
		t.Errorf("Identity.ID: got %s, want retry-id", a.Identity.ID)
	}
}

func TestRegister_ConnectionError(t *testing.T) {
	cfg := testConfig(t, "http://localhost:59999")
	a := New(cfg)

	a.RetryConfig = RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     10 * time.Millisecond,
		Multiplier:   2.0,
	}

	err := a.Register(context.Background())
	if err == nil {
		t.Error("expected error for connection failure")
	}
}

func TestRegister_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(protocol.RegisterResponse{
			AgentID: "test-id",
			Secret:  "test-secret",
		})
	}))
	defer server.Close()

	a := New(testConfig(t, server.URL))
	a.RetryConfig = RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     10 * time.Millisecond,
		Multiplier:   2.0,
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	a.cancel()

	err := a.Register(ctx)
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

func TestRegister_PayloadStructure(t *testing.T) {
	var receivedReq protocol.RegisterRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&receivedReq); err != nil {
			t.Errorf("failed to decode payload: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(protocol.RegisterResponse{
			AgentID: "test-id",
			Secret:  "test-secret",
		})
	}))
	defer server.Close()

	a := New(testConfig(t, server.URL))
	a.Config.RegistrationToken = "my-token"

	err := a.Register(context.Background())
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if receivedReq.Token != "my-token" {
		t.Errorf("Token: got %s, want my-token", receivedReq.Token)
	}
	if receivedReq.Info.Hostname != "test-agent" {
		t.Errorf("Hostname: got %s, want test-agent", receivedReq.Info.Hostname)
	}
	if receivedReq.Info.OS == "" {
		t.Error("OS should not be empty")
	}
	if receivedReq.Info.Arch == "" {
		t.Error("Arch should not be empty")
	}
}

func TestRegister_UserAgent(t *testing.T) {
	var receivedUA string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedUA = r.Header.Get("User-Agent")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(protocol.RegisterResponse{
			AgentID: "test-id",
			Secret:  "test-secret",
		})
	}))
	defer server.Close()

	a := New(testConfig(t, server.URL))

	a.Register(context.Background())
	if receivedUA != "Spectra-Agent/1.0" {
		t.Errorf("User-Agent: got %s, want Spectra-Agent/1.0", receivedUA)
	}
}

func TestRegister_BadRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	a := New(testConfig(t, server.URL))

	err := a.Register(context.Background())
	if err == nil {
		t.Error("expected error for 400 response")
	}
}

func TestRegister_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	a := New(testConfig(t, server.URL))

	err := a.Register(context.Background())
	if err == nil {
		t.Error("expected error for 401 response")
	}
}

func TestRegister_SavesIdentity(t *testing.T) {
	server := fakeRegisterServer(t)
	defer server.Close()

	a := New(testConfig(t, server.URL))

	err := a.Register(context.Background())
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if _, err := os.Stat(a.Config.IdentityPath); os.IsNotExist(err) {
		t.Errorf("identity file should exist at %s", a.Config.IdentityPath)
	}
}

func TestIdentity_LoadSaveRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent-id.json")

	original := Identity{
		ID:     "roundtrip-id",
		Secret: "roundtrip-secret",
	}

	err := saveIdentity(original, path)
	if err != nil {
		t.Fatalf("saveIdentity failed: %v", err)
	}

	loaded, err := loadIdentity(path)
	if err != nil {
		t.Fatalf("loadIdentity failed: %v", err)
	}

	if loaded.ID != original.ID {
		t.Errorf("ID: got %s, want %s", loaded.ID, original.ID)
	}
	if loaded.Secret != original.Secret {
		t.Errorf("Secret: got %s, want %s", loaded.Secret, original.Secret)
	}
}

func TestLoadIdentity_NotFound(t *testing.T) {
	path := filepath.Join(t.TempDir(), "non-existent-file.json")

	_, err := loadIdentity(path)
	if err == nil {
		t.Error("expected error when identity file doesn't exist")
	}
}

func BenchmarkRegister(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(protocol.RegisterResponse{
			AgentID: "bench-id",
			Secret:  "bench-secret",
		})
	}))
	defer server.Close()

	a := New(benchConfig(b, server.URL))

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		a.Register(context.Background())
	}
}
