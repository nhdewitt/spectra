package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nhdewitt/spectra/internal/database"
	"github.com/nhdewitt/spectra/internal/secret"
)

// testCipher builds a secret.Cipher with a throwaway key for handler tests.
func testCipher(t *testing.T) *secret.Cipher {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	c, err := secret.New(key)
	if err != nil {
		t.Fatalf("build test cipher: %v", err)
	}
	return c
}

func TestHandleGetSMTPConfig_NotConfigured(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSessionWithRole(mock, testSessionToken, "admin", "admin", testSessionIP, pgtype.UUID{})

	req := authedRequest(httptest.NewRequest(http.MethodGet, "/api/v1/admin/smtp", nil))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	var resp smtpConfigResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Enabled {
		t.Error("expected disabled default")
	}
	if resp.TLSMode != string(SMTPTLSStartTLS) {
		t.Errorf("default tls_mode: got %q, want starttls", resp.TLSMode)
	}
	if resp.PasswordSet {
		t.Error("expected password_set false when not configured")
	}
}

func TestHandleGetSMTPConfig_RedactsPassword(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSessionWithRole(mock, testSessionToken, "admin", "admin", testSessionIP, pgtype.UUID{})
	mock.SMTPConfig = &database.SmtpConfig{
		ID:                true,
		Enabled:           true,
		Host:              "smtp.example.com",
		Port:              587,
		Username:          "user",
		PasswordEncrypted: "spectra.v1:somethingencrypted",
		FromAddress:       "alerts@example.com",
		TlsMode:           "starttls",
	}

	req := authedRequest(httptest.NewRequest(http.MethodGet, "/api/v1/admin/smtp", nil))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	// The raw response must not contain the encrypted password anywhere.
	if strings.Contains(rec.Body.String(), "somethingencrypted") {
		t.Error("response leaked the stored password")
	}
	var resp smtpConfigResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if !resp.PasswordSet {
		t.Error("expected password_set true")
	}
}

func TestHandleUpdateSMTPConfig_EncryptsNewPassword(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSessionWithRole(mock, testSessionToken, "admin", "admin", testSessionIP, pgtype.UUID{})
	s.Cipher = testCipher(t)

	body := jsonBody(t, smtpConfigRequest{
		Enabled:     true,
		Host:        "smtp.example.com",
		Port:        587,
		Username:    "user",
		Password:    new("supersecret"),
		FromAddress: "alerts@example.com",
		TLSMode:     "starttls",
	})
	req := authedRequest(httptest.NewRequest(http.MethodPut, "/api/v1/admin/smtp", body))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	// Stored password must be encrypted (scheme-prefixed), not plaintext.
	stored := mock.SMTPConfig.PasswordEncrypted
	if stored == "supersecret" {
		t.Fatal("password stored in plaintext")
	}
	if !strings.HasPrefix(stored, "spectra.v1:") {
		t.Errorf("stored password not in expected encrypted format: %q", stored)
	}
	// And it must round-trip back to the original.
	got, err := s.Cipher.DecryptString(stored)
	if err != nil || got != "supersecret" {
		t.Errorf("decrypt round-trip failed: got %q err %v", got, err)
	}
}

func TestHandleUpdateSMTPConfig_NilPasswordPreservesStored(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSessionWithRole(mock, testSessionToken, "admin", "admin", testSessionIP, pgtype.UUID{})
	s.Cipher = testCipher(t)
	mock.SMTPConfig = &database.SmtpConfig{
		ID:                true,
		Enabled:           true,
		Host:              "old.example.com",
		Port:              587,
		PasswordEncrypted: "spectra.v1:preexisting",
		FromAddress:       "alerts@example.com",
		TlsMode:           "starttls",
	}

	// Password omitted (nil) — change only the host.
	body := jsonBody(t, smtpConfigRequest{
		Enabled:     true,
		Host:        "new.example.com",
		Port:        587,
		FromAddress: "alerts@example.com",
		TLSMode:     "starttls",
		Password:    nil,
	})
	req := authedRequest(httptest.NewRequest(http.MethodPut, "/api/v1/admin/smtp", body))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	if mock.SMTPConfig.PasswordEncrypted != "spectra.v1:preexisting" {
		t.Errorf("nil password should preserve stored value, got %q", mock.SMTPConfig.PasswordEncrypted)
	}
	if mock.SMTPConfig.Host != "new.example.com" {
		t.Errorf("host not updated: %q", mock.SMTPConfig.Host)
	}
}

func TestHandleUpdateSMTPConfig_EmptyPasswordClears(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSessionWithRole(mock, testSessionToken, "admin", "admin", testSessionIP, pgtype.UUID{})
	s.Cipher = testCipher(t)
	mock.SMTPConfig = &database.SmtpConfig{
		ID:                true,
		Enabled:           true,
		Host:              "smtp.example.com",
		Port:              587,
		PasswordEncrypted: "spectra.v1:preexisting",
		FromAddress:       "alerts@example.com",
		TlsMode:           "starttls",
	}

	body := jsonBody(t, smtpConfigRequest{
		Enabled:     true,
		Host:        "smtp.example.com",
		Port:        587,
		FromAddress: "alerts@example.com",
		TLSMode:     "starttls",
		Password:    new(""), // explicit clear
	})
	req := authedRequest(httptest.NewRequest(http.MethodPut, "/api/v1/admin/smtp", body))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	if mock.SMTPConfig.PasswordEncrypted != "" {
		t.Errorf("empty password should clear stored value, got %q", mock.SMTPConfig.PasswordEncrypted)
	}
}

func TestHandleUpdateSMTPConfig_DisabledAllowsIncomplete(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSessionWithRole(mock, testSessionToken, "admin", "admin", testSessionIP, pgtype.UUID{})
	s.Cipher = testCipher(t)

	// Disabled + blank host/from should be accepted.
	body := jsonBody(t, smtpConfigRequest{
		Enabled: false,
		TLSMode: "starttls",
	})
	req := authedRequest(httptest.NewRequest(http.MethodPut, "/api/v1/admin/smtp", body))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("disabled incomplete config should save: got %d, want 200", rec.Code)
	}
}

func TestHandleUpdateSMTPConfig_EnabledRequiresComplete(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSessionWithRole(mock, testSessionToken, "admin", "admin", testSessionIP, pgtype.UUID{})
	s.Cipher = testCipher(t)

	// Enabled but missing host → 400.
	body := jsonBody(t, smtpConfigRequest{
		Enabled:     true,
		Port:        587,
		FromAddress: "alerts@example.com",
		TLSMode:     "starttls",
	})
	req := authedRequest(httptest.NewRequest(http.MethodPut, "/api/v1/admin/smtp", body))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("enabled incomplete config should be rejected: got %d, want 400", rec.Code)
	}
}

func TestHandleUpdateSMTPConfig_NewPasswordNoCipher(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSessionWithRole(mock, testSessionToken, "admin", "admin", testSessionIP, pgtype.UUID{})
	s.Cipher = nil // no encryption key configured

	body := jsonBody(t, smtpConfigRequest{
		Enabled:     true,
		Host:        "smtp.example.com",
		Port:        587,
		Password:    new("secret"),
		FromAddress: "alerts@example.com",
		TLSMode:     "starttls",
	})
	req := authedRequest(httptest.NewRequest(http.MethodPut, "/api/v1/admin/smtp", body))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("setting password without cipher should be 503: got %d", rec.Code)
	}
}

func TestHandleUpdateSMTPConfig_InvalidTLSMode(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSessionWithRole(mock, testSessionToken, "admin", "admin", testSessionIP, pgtype.UUID{})
	s.Cipher = testCipher(t)

	body := jsonBody(t, smtpConfigRequest{
		Enabled:     true,
		Host:        "smtp.example.com",
		Port:        587,
		FromAddress: "alerts@example.com",
		TLSMode:     "carrier-pigeon",
	})
	req := authedRequest(httptest.NewRequest(http.MethodPut, "/api/v1/admin/smtp", body))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("invalid tls_mode should be 400: got %d", rec.Code)
	}
}

func TestHandleSMTPConfig_RequiresAdmin(t *testing.T) {
	s, _, _, mock := newTestServer()
	// Viewer role — below admin.
	setupTestSessionWithRole(mock, testSessionToken, "viewer", "viewer", testSessionIP, pgtype.UUID{})

	req := authedRequest(httptest.NewRequest(http.MethodGet, "/api/v1/admin/smtp", nil))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("non-admin should be forbidden: got %d, want 403", rec.Code)
	}
}

func TestResolvePassword_States(t *testing.T) {
	s, _, _, mock := newTestServer()
	s.Cipher = testCipher(t)
	mock.SMTPConfig = &database.SmtpConfig{
		ID:                true,
		PasswordEncrypted: "spectra.v1:stored",
	}
	ctx := context.Background()

	t.Run("nil preserves", func(t *testing.T) {
		got, err := s.resolvePassword(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		if got != "spectra.v1:stored" {
			t.Errorf("got %q, want stored value", got)
		}
	})

	t.Run("empty clears", func(t *testing.T) {
		got, err := s.resolvePassword(ctx, new(""))
		if err != nil {
			t.Fatal(err)
		}
		if got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})

	t.Run("value encrypts", func(t *testing.T) {
		got, err := s.resolvePassword(ctx, new("newpass"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(got, "spectra.v1:") {
			t.Errorf("expected encrypted value, got %q", got)
		}
		dec, _ := s.Cipher.DecryptString(got)
		if dec != "newpass" {
			t.Errorf("round-trip mismatch: %q", dec)
		}
	})
}
