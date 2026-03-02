package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// --- Admin Triggers ---

func TestHandleAdminTriggerLogs_Success(t *testing.T) {
	s, agentID, _, mock := newTestServer()
	setupTestSession(mock)

	req := authedRequest(httptest.NewRequest(http.MethodPost, "/api/v1/admin/logs?agent="+agentID, nil))
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestHandleAdminTriggerLogs_Unauthenticated(t *testing.T) {
	s, agentID, _, _ := newTestServer()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/logs?agent="+agentID, nil)
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
}

func TestHandleAdminTriggerLogs_MissingAgent(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	req := authedRequest(httptest.NewRequest(http.MethodPost, "/api/v1/admin/logs", nil))
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestHandleAdminTriggerLogs_UnknownAgent(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	req := authedRequest(httptest.NewRequest(http.MethodPost, "/api/v1/admin/logs?agent=nonexistent", nil))
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", rec.Code)
	}
}

func TestHandleAdminTriggerDisk_Success(t *testing.T) {
	s, agentID, _, mock := newTestServer()
	setupTestSession(mock)

	req := authedRequest(httptest.NewRequest(http.MethodPost, "/api/v1/admin/disk?agent="+agentID+"&path=/&top_n=10", nil))
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestHandleAdminTriggerDisk_Unauthenticated(t *testing.T) {
	s, agentID, _, _ := newTestServer()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/disk?agent="+agentID+"&path=/&top_n=10", nil)
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
}

func TestHandleAdminTriggerDisk_MissingAgent(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	req := authedRequest(httptest.NewRequest(http.MethodPost, "/api/v1/admin/disk", nil))
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestHandleAdminTriggerNetwork_Success(t *testing.T) {
	s, agentID, _, mock := newTestServer()
	setupTestSession(mock)

	req := authedRequest(httptest.NewRequest(http.MethodPost, "/api/v1/admin/network?agent="+agentID+"&action=ping&target=8.8.8.8", nil))
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestHandleAdminTriggerNetwork_Unauthenticated(t *testing.T) {
	s, agentID, _, _ := newTestServer()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/network?agent="+agentID+"&action=ping&target=8.8.8.8", nil)
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
}

func TestHandleAdminTriggerNetwork_MissingAgent(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	req := authedRequest(httptest.NewRequest(http.MethodPost, "/api/v1/admin/network?action=ping", nil))
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestHandleAdminTriggerNetwork_MissingAction(t *testing.T) {
	s, agentID, _, mock := newTestServer()
	setupTestSession(mock)

	req := authedRequest(httptest.NewRequest(http.MethodPost, "/api/v1/admin/network?agent="+agentID, nil))
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

// --- Token Generation ---

func TestHandleGenerateToken_Success(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	req := authedRequest(httptest.NewRequest(http.MethodPost, "/api/v1/admin/tokens", nil))
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status: got %d, want 201", rec.Code)
	}
}

func TestHandleGenerateToken_Unauthenticated(t *testing.T) {
	s, _, _, _ := newTestServer()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/tokens", nil)
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
}
