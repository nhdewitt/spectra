package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleLogin_Success(t *testing.T) {
	s, _, _, mock := newTestServer()
	mock.AddUser("admin", "password123", "admin")

	body, _ := json.Marshal(map[string]string{
		"username": "admin",
		"password": "password123",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.RemoteAddr = "192.168.1.1:12345"
	rec := httptest.NewRecorder()

	s.handleLogin(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}

	cookies := rec.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == sessionCookieName {
			found = true
			if !c.HttpOnly {
				t.Error("session cookie should be HttpOnly")
			}
			if c.SameSite != http.SameSiteStrictMode {
				t.Error("session cookie should be SameSite=Strict")
			}
		}
	}

	if !found {
		t.Error("response should set session cookie")
	}
}

func TestHandleLogin_WrongPassword(t *testing.T) {
	s, _, _, mock := newTestServer()
	mock.AddUser("admin", "password123", "admin")

	body, _ := json.Marshal(map[string]string{
		"username": "admin",
		"password": "wrongpass",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.RemoteAddr = "192.168.1.1:12345"
	rec := httptest.NewRecorder()

	s.handleLogin(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
}

func TestHandleLogin_UnknownUser(t *testing.T) {
	s, _, _, _ := newTestServer()

	body, _ := json.Marshal(map[string]string{
		"username": "nobody",
		"password": "password",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.RemoteAddr = "192.168.1.1:12345"
	rec := httptest.NewRecorder()

	s.handleLogin(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
}

func TestHandleLogin_EmptyFields(t *testing.T) {
	s, _, _, _ := newTestServer()

	body, _ := json.Marshal(map[string]string{
		"username": "",
		"password": "",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.RemoteAddr = "192.168.1.1:12345"
	rec := httptest.NewRecorder()

	s.handleLogin(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestHandleLogout(t *testing.T) {
	s, _, _, _ := newTestServer()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "some-token"})
	rec := httptest.NewRecorder()

	s.handleLogout(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status: got %d, want 204", rec.Code)
	}

	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookieName && c.MaxAge != -1 {
			t.Error("session cookie should be expired")
		}
	}
}

func TestHandleLogout_NoCookie(t *testing.T) {
	s, _, _, _ := newTestServer()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	rec := httptest.NewRecorder()

	s.handleLogout(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status: got %d, want 204", rec.Code)
	}
}

func TestRequireUserAuth_ValidSession(t *testing.T) {
	s, _, _, mock := newTestServer()
	mock.AddSession("valid-token", "admin", "admin", "192.168.1.1")

	handler := s.requireUserAuth(func(w http.ResponseWriter, r *http.Request) {
		u, ok := userFromContext(r.Context())
		if !ok {
			t.Error("expected user in context")
			return
		}
		if u.Username != "admin" {
			t.Errorf("username = %s, want admin", u.Username)
		}
		if u.Role != "admin" {
			t.Errorf("role = %s, want admin", u.Role)
		}
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "valid-token"})
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestRequireUserAuth_NoCookie(t *testing.T) {
	s, _, _, _ := newTestServer()

	handler := s.requireUserAuth(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
}

func TestRequireUserAuth_InvalidToken(t *testing.T) {
	s, _, _, _ := newTestServer()

	handler := s.requireUserAuth(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "invalid-token"})
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
}

func TestRequireUserAuth_IPMismatch(t *testing.T) {
	s, _, _, mock := newTestServer()
	mock.AddSession("valid-token", "admin", "admin", "192.168.1.1")

	handler := s.requireUserAuth(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called on IP mismatch")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.99:54321"
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "valid-token"})
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
}

func TestHandleMe_Authenticated(t *testing.T) {
	s, _, _, _ := newTestServer()

	handler := s.requireUserAuth(s.handleMe)

	s.DB.(*MockDB).AddSession("me-token", "testuser", "admin", "192.168.1.1")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "me-token"})
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}

	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["username"] != "testuser" {
		t.Errorf("username = %s, want testuser", resp["username"])
	}
}

func TestHandleMe_Unauthenticated(t *testing.T) {
	s, _, _, _ := newTestServer()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	s.handleMe(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
}

func TestUserFromContext_NotSet(t *testing.T) {
	ctx := context.Background()
	_, ok := userFromContext(ctx)
	if ok {
		t.Error("expected false for empty context")
	}
}

func TestUserFromContext_Set(t *testing.T) {
	u := &userContext{ID: "123", Username: "test", Role: "admin"}
	ctx := context.WithValue(context.Background(), userContextKey, u)

	got, ok := userFromContext(ctx)
	if !ok {
		t.Fatal("expected true")
	}
	if got.Username != "test" {
		t.Errorf("username = %s, want test", got.Username)
	}
}

func TestLoginTracker_AllowsInitial(t *testing.T) {
	lt := newLoginTracker()
	if err := lt.check("192.168.1.1"); err != nil {
		t.Errorf("initial check should pass: %v", err)
	}
}

func TestLoginTracker_LocksAfterMaxAttempts(t *testing.T) {
	lt := newLoginTracker()

	for range maxLoginAttempts {
		lt.recordFailure("192.168.1.1")
	}

	if err := lt.check("192.168.1.1"); err == nil {
		t.Error("should be locked after max attempts")
	}
}

func TestLoginTracker_SeparateIPs(t *testing.T) {
	lt := newLoginTracker()

	for range maxLoginAttempts {
		lt.recordFailure("192.168.1.1")
	}

	// Different IP should be fine
	if err := lt.check("192.168.1.2"); err != nil {
		t.Errorf("different IP should not be locked: %v", err)
	}
}

func TestLoginTracker_SuccessResetsCount(t *testing.T) {
	lt := newLoginTracker()

	for range maxLoginAttempts - 1 {
		lt.recordFailure("192.168.1.1")
	}

	lt.recordSuccess("192.168.1.1")

	// Should be reset, more failures needed to lock
	for range maxLoginAttempts - 1 {
		lt.recordFailure("192.168.1.1")
	}

	if err := lt.check("192.168.1.1"); err != nil {
		t.Errorf("should not be locked after reset: %v", err)
	}
}

func TestLoginTracker_HandleLogin_Lockout(t *testing.T) {
	s, _, _, mock := newTestServer()
	mock.AddUser("admin", "password123", "admin")

	// Exhaust login attempts
	for range maxLoginAttempts {
		body, _ := json.Marshal(map[string]string{
			"username": "admin",
			"password": "wrongpass",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
		req.RemoteAddr = "192.168.1.1:12345"
		rec := httptest.NewRecorder()
		s.handleLogin(rec, req)
	}

	// Next attempt should be locked even with the correct password
	body, _ := json.Marshal(map[string]string{
		"username": "admin",
		"password": "password123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.RemoteAddr = "192.168.1.1:12345"
	rec := httptest.NewRecorder()
	s.handleLogin(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status: got %d, want 429", rec.Code)
	}
}

func BenchmarkRequireUserAuth_ValidSession(b *testing.B) {
	s, _, _, mock := newTestServer()
	mock.AddSession("bench-token", "admin", "admin", "192.168.1.1")

	handler := s.requireUserAuth(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "bench-token"})
		rec := httptest.NewRecorder()
		handler(rec, req)
	}
}

func BenchmarkLoginTracker_Check(b *testing.B) {
	lt := newLoginTracker()

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		lt.check("192.168.1.1")
	}
}
