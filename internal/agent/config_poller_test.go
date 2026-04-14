package agent

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nhdewitt/spectra/internal/logging"
)

func newTestAgentWithLogger() *Agent {
	a := New(Config{
		BaseURL:     "http://localhost:8080",
		Hostname:    "test-host",
		MetricsPath: "/api/v1/agent/metrics",
		CommandPath: "/api/v1/agent/command",
	})
	a.Logger = logging.New(logging.Config{
		ConsoleLevel: slog.LevelError, // suppress noise in tests
	})
	a.Identity = Identity{
		ID:     "550e8400-e29b-41d4-a716-446655440000",
		Secret: "test-secret",
	}
	return a
}

func TestFetchAndApplyConfig_LogLevel(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		config := map[string]json.RawMessage{
			"log_level": json.RawMessage(`"debug"`),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(config)
	})
	srv := httptest.NewServer(handler)
	defer srv.Close()

	a := newTestAgentWithLogger()
	a.Config.BaseURL = srv.URL

	// Verify initial level
	if a.Logger.ConsoleLevel.Level() != slog.LevelError {
		t.Fatalf("expected initial level Error, got %v", a.Logger.ConsoleLevel.Level())
	}

	a.fetchAndApplyConfig(context.Background())

	if a.Logger.ConsoleLevel.Level() != slog.LevelDebug {
		t.Errorf("expected console level Debug after config apply, got %v", a.Logger.ConsoleLevel.Level())
	}
	if a.Logger.FileLevel.Level() != slog.LevelDebug {
		t.Errorf("expected file level Debug after config apply, got %v", a.Logger.FileLevel.Level())
	}
}

func TestFetchAndApplyConfig_WarnLevel(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		config := map[string]json.RawMessage{
			"log_level": json.RawMessage(`"warn"`),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(config)
	})
	srv := httptest.NewServer(handler)
	defer srv.Close()

	a := newTestAgentWithLogger()
	a.Config.BaseURL = srv.URL

	a.fetchAndApplyConfig(context.Background())

	if a.Logger.ConsoleLevel.Level() != slog.LevelWarn {
		t.Errorf("expected console level Warn, got %v", a.Logger.ConsoleLevel.Level())
	}
}

func TestFetchAndApplyConfig_EmptyConfig(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]json.RawMessage{})
	})
	srv := httptest.NewServer(handler)
	defer srv.Close()

	a := newTestAgentWithLogger()
	a.Config.BaseURL = srv.URL

	initialLevel := a.Logger.ConsoleLevel.Level()
	a.fetchAndApplyConfig(context.Background())

	// Level should not change
	if a.Logger.ConsoleLevel.Level() != initialLevel {
		t.Errorf("expected level unchanged at %v, got %v", initialLevel, a.Logger.ConsoleLevel.Level())
	}
}

func TestFetchAndApplyConfig_NoLogLevelKey(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		config := map[string]json.RawMessage{
			"ignored_filesystems": json.RawMessage(`["nfs"]`),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(config)
	})
	srv := httptest.NewServer(handler)
	defer srv.Close()

	a := newTestAgentWithLogger()
	a.Config.BaseURL = srv.URL

	initialLevel := a.Logger.ConsoleLevel.Level()
	a.fetchAndApplyConfig(context.Background())

	if a.Logger.ConsoleLevel.Level() != initialLevel {
		t.Errorf("expected level unchanged, got %v", a.Logger.ConsoleLevel.Level())
	}
}

func TestFetchAndApplyConfig_EmptyLogLevel(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		config := map[string]json.RawMessage{
			"log_level": json.RawMessage(`""`),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(config)
	})
	srv := httptest.NewServer(handler)
	defer srv.Close()

	a := newTestAgentWithLogger()
	a.Config.BaseURL = srv.URL

	initialLevel := a.Logger.ConsoleLevel.Level()
	a.fetchAndApplyConfig(context.Background())

	// Empty string should not change level
	if a.Logger.ConsoleLevel.Level() != initialLevel {
		t.Errorf("expected level unchanged for empty string, got %v", a.Logger.ConsoleLevel.Level())
	}
}

func TestFetchAndApplyConfig_ServerError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	srv := httptest.NewServer(handler)
	defer srv.Close()

	a := newTestAgentWithLogger()
	a.Config.BaseURL = srv.URL

	initialLevel := a.Logger.ConsoleLevel.Level()

	// Should not panic or change anything
	a.fetchAndApplyConfig(context.Background())

	if a.Logger.ConsoleLevel.Level() != initialLevel {
		t.Errorf("expected level unchanged on server error, got %v", a.Logger.ConsoleLevel.Level())
	}
}

func TestFetchAndApplyConfig_Unauthorized(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	srv := httptest.NewServer(handler)
	defer srv.Close()

	a := newTestAgentWithLogger()
	a.Config.BaseURL = srv.URL

	initialLevel := a.Logger.ConsoleLevel.Level()
	a.fetchAndApplyConfig(context.Background())

	if a.Logger.ConsoleLevel.Level() != initialLevel {
		t.Errorf("expected level unchanged on 401, got %v", a.Logger.ConsoleLevel.Level())
	}
}

func TestFetchAndApplyConfig_InvalidJSON(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not json at all"))
	})
	srv := httptest.NewServer(handler)
	defer srv.Close()

	a := newTestAgentWithLogger()
	a.Config.BaseURL = srv.URL

	initialLevel := a.Logger.ConsoleLevel.Level()

	// Should not panic
	a.fetchAndApplyConfig(context.Background())

	if a.Logger.ConsoleLevel.Level() != initialLevel {
		t.Errorf("expected level unchanged on bad JSON, got %v", a.Logger.ConsoleLevel.Level())
	}
}

func TestFetchAndApplyConfig_ServerDown(t *testing.T) {
	a := newTestAgentWithLogger()
	a.Config.BaseURL = "http://127.0.0.1:1" // nothing listening

	initialLevel := a.Logger.ConsoleLevel.Level()

	// Should not panic
	a.fetchAndApplyConfig(context.Background())

	if a.Logger.ConsoleLevel.Level() != initialLevel {
		t.Errorf("expected level unchanged when server unreachable, got %v", a.Logger.ConsoleLevel.Level())
	}
}

func TestFetchAndApplyConfig_CorrectURL(t *testing.T) {
	var capturedPath string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]json.RawMessage{})
	})
	srv := httptest.NewServer(handler)
	defer srv.Close()

	a := newTestAgentWithLogger()
	a.Config.BaseURL = srv.URL
	a.Identity.ID = "abc-123-def"

	a.fetchAndApplyConfig(context.Background())

	expected := "/api/v1/agents/abc-123-def/config"
	if capturedPath != expected {
		t.Errorf("expected path %q, got %q", expected, capturedPath)
	}
}

func TestFetchAndApplyConfig_SetsAuthHeaders(t *testing.T) {
	var capturedAgentID, capturedSecret string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAgentID = r.Header.Get("X-Agent-ID")
		capturedSecret = r.Header.Get("X-Agent-Secret")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]json.RawMessage{})
	})
	srv := httptest.NewServer(handler)
	defer srv.Close()

	a := newTestAgentWithLogger()
	a.Config.BaseURL = srv.URL

	a.fetchAndApplyConfig(context.Background())

	if capturedAgentID != a.Identity.ID {
		t.Errorf("expected X-Agent-ID %q, got %q", a.Identity.ID, capturedAgentID)
	}
	if capturedSecret != a.Identity.Secret {
		t.Errorf("expected X-Agent-Secret %q, got %q", a.Identity.Secret, capturedSecret)
	}
}

func TestFetchAndApplyConfig_NoContentEncoding(t *testing.T) {
	var capturedEncoding string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedEncoding = r.Header.Get("Content-Encoding")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]json.RawMessage{})
	})
	srv := httptest.NewServer(handler)
	defer srv.Close()

	a := newTestAgentWithLogger()
	a.Config.BaseURL = srv.URL

	a.fetchAndApplyConfig(context.Background())

	if capturedEncoding != "" {
		t.Errorf("expected no Content-Encoding header, got %q", capturedEncoding)
	}
}
