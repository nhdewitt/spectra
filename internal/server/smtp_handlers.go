package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/nhdewitt/spectra/internal/database"
)

// errEncryptionNotConfigured indicates a password operation was attempted but
// no encryption key is available.
var errEncryptionNotConfigured = errors.New("encryption key not configured")

// smtpConfigResponse is the GET/PUT response shape. The password is never
// returned; passwordSet tells the UI whether one is stored so it can render
// "password configured" without exposing it.
type smtpConfigResponse struct {
	Enabled     bool      `json:"enabled"`
	Host        string    `json:"host"`
	Port        int32     `json:"port"`
	Username    string    `json:"username"`
	PasswordSet bool      `json:"password_set"`
	FromAddress string    `json:"from_address"`
	TLSMode     string    `json:"tls_mode"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func toSMTPConfigResponse(c database.SmtpConfig) smtpConfigResponse {
	return smtpConfigResponse{
		Enabled:     c.Enabled,
		Host:        c.Host,
		Port:        c.Port,
		Username:    c.Username,
		PasswordSet: c.PasswordEncrypted != "",
		FromAddress: c.FromAddress,
		TLSMode:     c.TlsMode,
		UpdatedAt:   c.UpdatedAt.Time,
	}
}

// smtpConfigRequest is the PUT/test request body.
//
// Password is a pointer to distinguish three intents:
//
//	nil		-> leave the stored password unchanged
//	""		-> explicitly clear the stored password
//	"value"	-> set a new password (encrypted on write)
type smtpConfigRequest struct {
	Enabled     bool    `json:"enabled"`
	Host        string  `json:"host"`
	Port        int32   `json:"port"`
	Username    string  `json:"username"`
	Password    *string `json:"password"`
	FromAddress string  `json:"from_address"`
	TLSMode     string  `json:"tls_mode"`
	TestTo      string  `json:"test_to,omitempty"`
}

// normalize trims surrounding whitespace from all fields except the password.
func (req *smtpConfigRequest) normalize() {
	req.Host = strings.TrimSpace(req.Host)
	req.Username = strings.TrimSpace(req.Username)
	req.FromAddress = strings.TrimSpace(req.FromAddress)
	req.TLSMode = strings.TrimSpace(req.TLSMode)
	req.TestTo = strings.TrimSpace(req.TestTo)
}

// validateSMTPConfigForSave validates only when SMTP is being enabled; a
// disabled config may be saved incomplete. Returns the parsed TLS mode so
// callers use the validated value rather than re-casting the raw string.
func validateSMTPConfigForSave(req smtpConfigRequest) (SMTPTLSMode, error) {
	if !req.Enabled {
		if req.TLSMode == "" {
			return SMTPTLSStartTLS, nil
		}
		return ParseSMTPTLSMode(req.TLSMode)
	}
	return validateSMTPConfigComplete(req)
}

// validateSMTPConfigComplete requires every field needed to actually send.
// Used for enabled saves and for the test endpoint.
func validateSMTPConfigComplete(req smtpConfigRequest) (SMTPTLSMode, error) {
	if req.Host == "" {
		return "", errors.New("host is required")
	}
	if req.Port < 1 || req.Port > 65535 {
		return "", errors.New("port must be between 1 and 65535")
	}
	if req.FromAddress == "" {
		return "", errors.New("from_address is required")
	}
	return ParseSMTPTLSMode(req.TLSMode)
}

// resolvePassword returns the encrypted password to store.
func (s *Server) resolvePassword(ctx context.Context, reqPassword *string) (string, error) {
	if reqPassword == nil {
		// Unchanged, retain whatever is stored
		existing, err := s.DB.GetSMTPConfig(ctx)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return "", nil
			}
			return "", err
		}
		return existing.PasswordEncrypted, nil
	}
	if *reqPassword == "" {
		// Explicit clear
		return "", nil
	}
	if s.Cipher == nil {
		return "", fmt.Errorf("cannot set SMTP password: %w", errEncryptionNotConfigured)
	}
	enc, err := s.Cipher.EncryptString(*reqPassword)
	if err != nil {
		return "", fmt.Errorf("encrypt password: %w", err)
	}
	return enc, nil
}

// handleGetSMTPConfig returns the current SMTP config with the password redacted.
//
// GET /api/v1/admin/smtp
func (s *Server) handleGetSMTPConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.DB.GetSMTPConfig(r.Context())
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Not configured yet
			respondJSON(w, http.StatusOK, smtpConfigResponse{TLSMode: string(SMTPTLSStartTLS)})
			return
		}
		s.dbError(w, err, "handleGetSMTPConfig")
		return
	}
	respondJSON(w, http.StatusOK, toSMTPConfigResponse(cfg))
}

// handleUpdateSMTPConfig upserts the SMTP config. The password is encrypted on write;
// a nil password preserved the stored value, an empty password clears it.
//
// PUT /api/v1/admin/smtp
func (s *Server) handleUpdateSMTPConfig(w http.ResponseWriter, r *http.Request) {
	var req smtpConfigRequest
	if err := decodeJSONBody(r, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req.normalize()

	tlsMode, err := validateSMTPConfigForSave(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	encPassword, err := s.resolvePassword(r.Context(), req.Password)
	if err != nil {
		if errors.Is(err, errEncryptionNotConfigured) {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		s.dbError(w, err, "handleUpdateSMTPConfig")
		return
	}

	cfg, err := s.DB.UpsertSMTPConfig(r.Context(), database.UpsertSMTPConfigParams{
		Enabled:           req.Enabled,
		Host:              req.Host,
		Port:              req.Port,
		Username:          req.Username,
		PasswordEncrypted: encPassword,
		FromAddress:       req.FromAddress,
		TlsMode:           string(tlsMode),
	})
	if err != nil {
		s.dbError(w, err, "handleUpdateSMTPConfig")
		return
	}

	s.Logger.Info("smtp config updated", "enabled", cfg.Enabled, "host", cfg.Host)
	respondJSON(w, http.StatusOK, toSMTPConfigResponse(cfg))
}

// handleTestSMTPConfig sends a test email using the request-body settings,
// resolving the password from the body or the stored value. Nothing is saved -
// this validates a config before the admin commits it.
//
// POST /api/v1/admin/smtp/test
func (s *Server) handleTestSMTPConfig(w http.ResponseWriter, r *http.Request) {
	var req smtpConfigRequest
	if err := decodeJSONBody(r, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req.normalize()

	tlsMode, err := validateSMTPConfigComplete(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.TestTo == "" {
		http.Error(w, "test_to is required", http.StatusBadRequest)
		return
	}

	encPassword, err := s.resolvePassword(r.Context(), req.Password)
	if err != nil {
		if errors.Is(err, errEncryptionNotConfigured) {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		s.dbError(w, err, "handleTestSMTPConfig")
		return
	}

	var plainPassword string
	if encPassword != "" {
		if s.Cipher == nil {
			http.Error(w, errEncryptionNotConfigured.Error(), http.StatusServiceUnavailable)
			return
		}
		plainPassword, err = s.Cipher.DecryptString(encPassword)
		if err != nil {
			s.Logger.Error("decrypt SMTP password", "err", err)
			http.Error(w, "stored SMTP password could not be decrypted", http.StatusServiceUnavailable)
			return
		}
	}

	settings := smtpSettings{
		Host:     req.Host,
		Port:     int(req.Port),
		Username: req.Username,
		Password: plainPassword,
		From:     req.FromAddress,
		TLSMode:  tlsMode,
	}

	if err := s.sendTestEmail(r.Context(), settings, req.TestTo); err != nil {
		respondJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}
