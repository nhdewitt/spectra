package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleGetAgentConfig_Success(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	req := httptest.NewRequest("GET", "/api/v1/agents/"+testAgentUUID+"/config", nil)
	req = authedRequest(req)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var result map[string]json.RawMessage
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestHandleGetAgentConfig_InvalidID(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	req := httptest.NewRequest("GET", "/api/v1/agents/not-a-uuid/config", nil)
	req = authedRequest(req)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleGetAgentConfig_Unauthenticated(t *testing.T) {
	s, _, _, _ := newTestServer()

	req := httptest.NewRequest("GET", "/api/v1/agents/"+testAgentUUID+"/config", nil)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestHandleSetAgentConfig_Success(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	body := `{"key": "ignored_filesystems", "value": ["nfs", "cifs"]}`
	req := httptest.NewRequest("PUT", "/api/v1/agents/"+testAgentUUID+"/config", strings.NewReader(body))
	req = authedRequest(req)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleSetAgentConfig_LogLevel(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	body := `{"key": "log_level", "value": "debug"}`
	req := httptest.NewRequest("PUT", "/api/v1/agents/"+testAgentUUID+"/config", strings.NewReader(body))
	req = authedRequest(req)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleSetAgentConfig_InvalidKey(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	body := `{"key": "nonexistent_key", "value": "foo"}`
	req := httptest.NewRequest("PUT", "/api/v1/agents/"+testAgentUUID+"/config", strings.NewReader(body))
	req = authedRequest(req)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleSetAgentConfig_MissingKey(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	body := `{"key": "", "value": "foo"}`
	req := httptest.NewRequest("PUT", "/api/v1/agents/"+testAgentUUID+"/config", strings.NewReader(body))
	req = authedRequest(req)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleSetAgentConfig_InvalidJSON(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	req := httptest.NewRequest("PUT", "/api/v1/agents/"+testAgentUUID+"/config", strings.NewReader("not json"))
	req = authedRequest(req)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleSetAgentConfig_InvalidAgentID(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	body := `{"key": "labels", "value": {"env": "prod"}}`
	req := httptest.NewRequest("PUT", "/api/v1/agents/bad-id/config", strings.NewReader(body))
	req = authedRequest(req)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleSetAgentConfig_DBError(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)
	mock.ConfigErr = errFake

	body := `{"key": "labels", "value": {"env": "prod"}}`
	req := httptest.NewRequest("PUT", "/api/v1/agents/"+testAgentUUID+"/config", strings.NewReader(body))
	req = authedRequest(req)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandleSetAgentConfig_Unauthenticated(t *testing.T) {
	s, _, _, _ := newTestServer()

	body := `{"key": "labels", "value": {"env": "prod"}}`
	req := httptest.NewRequest("PUT", "/api/v1/agents/"+testAgentUUID+"/config", strings.NewReader(body))
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestHandleDeleteAgentConfig_Success(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	req := httptest.NewRequest("DELETE", "/api/v1/agents/"+testAgentUUID+"/config?key=labels", nil)
	req = authedRequest(req)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleDeleteAgentConfig_MissingKey(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	req := httptest.NewRequest("DELETE", "/api/v1/agents/"+testAgentUUID+"/config", nil)
	req = authedRequest(req)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleDeleteAgentConfig_InvalidAgentID(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	req := httptest.NewRequest("DELETE", "/api/v1/agents/bad-id/config?key=labels", nil)
	req = authedRequest(req)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleDeleteAgentConfig_DBError(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)
	mock.ConfigErr = errFake

	req := httptest.NewRequest("DELETE", "/api/v1/agents/"+testAgentUUID+"/config?key=labels", nil)
	req = authedRequest(req)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandleDeleteAgentConfig_Unauthenticated(t *testing.T) {
	s, _, _, _ := newTestServer()

	req := httptest.NewRequest("DELETE", "/api/v1/agents/"+testAgentUUID+"/config?key=labels", nil)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestHandleGetAgentSelfConfig(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		s, agentID, secret, _ := newTestServer()

		req := httptest.NewRequest(http.MethodGet, "/api/v1/agent/config", nil)
		setAgentAuth(req, agentID, secret)
		rec := httptest.NewRecorder()

		s.Router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status: got %d, want 200", rec.Code)
		}
	})

	t.Run("db_error", func(t *testing.T) {
		s, agentID, secret, mock := newTestServer()
		mock.QueryErr = errors.New("connection refused")

		req := httptest.NewRequest(http.MethodGet, "/api/v1/agent/config", nil)
		setAgentAuth(req, agentID, secret)
		rec := httptest.NewRecorder()

		s.Router.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("status: got %d, want 500", rec.Code)
		}
	})
}

func TestIsValidConfigKey(t *testing.T) {
	valid := []string{"ignored_filesystems", "ignored_interfaces", "labels", "log_level"}
	for _, k := range valid {
		if !isValidConfigKey(k) {
			t.Errorf("expected %q to be valid", k)
		}
	}

	invalid := []string{"", "foo", "password", "ignored_filesystem", "LOG_LEVEL"}
	for _, k := range invalid {
		if isValidConfigKey(k) {
			t.Errorf("expected %q to be invalid", k)
		}
	}
}
