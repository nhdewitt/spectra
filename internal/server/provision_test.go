package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleListPlatforms_NoReleases(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	req := httptest.NewRequest("GET", "/api/v1/admin/platforms", nil)
	req = authedRequest(req)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var result []platformInfo
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("expected empty list, got %d platforms", len(result))
	}
}

func TestHandleListPlatforms_Unauthenticated(t *testing.T) {
	s, _, _, _ := newTestServer()

	req := httptest.NewRequest("GET", "/api/v1/admin/platforms", nil)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestHandleProvision_Success(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	body := `{"platform": "spectra-agent-linux-amd64"}`
	req := httptest.NewRequest("POST", "/api/v1/admin/provision", strings.NewReader(body))
	req = authedRequest(req)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp provisionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if resp.Token == "" {
		t.Error("expected non-empty token")
	}
	if resp.Platform != "spectra-agent-linux-amd64" {
		t.Errorf("expected platform 'spectra-agent-linux-amd64', got %q", resp.Platform)
	}
	if resp.Config.Token == "" {
		t.Error("expected config.token to be set")
	}
	if resp.Config.Server == "" {
		t.Error("expected config.server to be set")
	}
}

func TestHandleProvision_MissingPlatform(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	body := `{"platform": ""}`
	req := httptest.NewRequest("POST", "/api/v1/admin/provision", strings.NewReader(body))
	req = authedRequest(req)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleProvision_UnknownPlatform(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	body := `{"platform": "spectra-agent-plan9-mips"}`
	req := httptest.NewRequest("POST", "/api/v1/admin/provision", strings.NewReader(body))
	req = authedRequest(req)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleProvision_InvalidJSON(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	req := httptest.NewRequest("POST", "/api/v1/admin/provision", strings.NewReader("not json"))
	req = authedRequest(req)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleProvision_Unauthenticated(t *testing.T) {
	s, _, _, _ := newTestServer()

	body := `{"platform": "spectra-agent-linux-amd64"}`
	req := httptest.NewRequest("POST", "/api/v1/admin/provision", strings.NewReader(body))
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestHandleDownloadConfig_Success(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	body := `{"server": "http://localhost:8080", "token": "abc123"}`
	req := httptest.NewRequest("POST", "/api/v1/admin/provision/config", strings.NewReader(body))
	req = authedRequest(req)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}

	cd := w.Header().Get("Content-Disposition")
	if !strings.Contains(cd, "spectra-agent.json") {
		t.Errorf("expected Content-Disposition with spectra-agent.json, got %q", cd)
	}
}

func TestHandleDownloadConfig_InvalidJSON(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	req := httptest.NewRequest("POST", "/api/v1/admin/provision/config", strings.NewReader("bad"))
	req = authedRequest(req)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleDownloadConfig_Unauthenticated(t *testing.T) {
	s, _, _, _ := newTestServer()

	body := `{"server": "http://localhost:8080", "token": "abc123"}`
	req := httptest.NewRequest("POST", "/api/v1/admin/provision/config", strings.NewReader(body))
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestHandleDownloadRelease_NoReleases(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	req := httptest.NewRequest("GET", "/api/v1/admin/releases/spectra-agent-linux-amd64", nil)
	req = authedRequest(req)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	// No releases configured, should 404
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestHandleDownloadRelease_PathTraversal(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	req := httptest.NewRequest("GET", "/api/v1/admin/releases/../../etc/passwd", nil)
	req = authedRequest(req)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	// Should reject path traversal
	if w.Code == http.StatusOK {
		t.Fatal("expected non-200 for path traversal attempt")
	}
}

func TestHandleDownloadRelease_Unauthenticated(t *testing.T) {
	s, _, _, _ := newTestServer()

	req := httptest.NewRequest("GET", "/api/v1/admin/releases/spectra-agent-linux-amd64", nil)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestHandleUpgradeInstructions_Success(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	req := httptest.NewRequest("GET", "/api/v1/agents/"+testAgentUUID+"/upgrade-instructions", nil)
	req = authedRequest(req)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 (no platform match for empty OS/arch), got %d", w.Code)
	}
}

func TestHandleUpgradeInstructions_InvalidID(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	req := httptest.NewRequest("GET", "/api/v1/agents/not-a-uuid/upgrade-instructions", nil)
	req = authedRequest(req)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleUpgradeInstructions_AgentNotFound(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)
	mock.GetAgentErr = errFake

	req := httptest.NewRequest("GET", "/api/v1/agents/"+testAgentUUID+"/upgrade-instructions", nil)
	req = authedRequest(req)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestHandleUpgradeInstructions_Unauthenticated(t *testing.T) {
	s, _, _, _ := newTestServer()

	req := httptest.NewRequest("GET", "/api/v1/agents/"+testAgentUUID+"/upgrade-instructions", nil)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestHandleUninstallInstructions_Success(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	req := httptest.NewRequest("GET", "/api/v1/agents/"+testAgentUUID+"/uninstall-instructions", nil)
	req = authedRequest(req)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 (no platform match for empty OS/arch), got %d", w.Code)
	}
}

func TestHandleUninstallInstructions_InvalidID(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	req := httptest.NewRequest("GET", "/api/v1/agents/not-a-uuid/uninstall-instructions", nil)
	req = authedRequest(req)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleUninstallInstructions_AgentNotFound(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)
	mock.GetAgentErr = errFake

	req := httptest.NewRequest("GET", "/api/v1/agents/"+testAgentUUID+"/uninstall-instructions", nil)
	req = authedRequest(req)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestHandleUninstallInstructions_Unauthenticated(t *testing.T) {
	s, _, _, _ := newTestServer()

	req := httptest.NewRequest("GET", "/api/v1/agents/"+testAgentUUID+"/uninstall-instructions", nil)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}
