package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

type installInstructions struct {
	Type    string `json:"type"`    // "systemd", "launchd", "windows_service", "rc_d"
	Content string `json:"content"` // unit file / plist / script
	Steps   string `json:"steps"`   // human-readable installation steps
}

type provisionRequest struct {
	Platform string `json:"platform"` // filename from /platforms (e.g. "spectra-agent-linux-amd64")
}

type provisionResponse struct {
	Token       string              `json:"token"`
	ExpiresAt   string              `json:"expires_at"`
	Platform    string              `json:"platform"`
	DownloadURL string              `json:"download_url"`
	CACert      string              `json:"ca_cert,omitempty"`
	Config      agentConfig         `json:"config"`
	Install     installInstructions `json:"install"`
}

type agentConfig struct {
	Server        string `json:"server"`
	Token         string `json:"token"`
	Interval      int    `json:"interval"`
	CACert        string `json:"ca_cert,omitempty"`
	TLSSkipVerify bool   `json:"tls_skip_verify,omitempty"`
}

// Handlers

// handleListPlatforms returns available agent builds that have verified binaries.
//
// GET /api/v1/admin/platforms
func (s *Server) handleListPlatforms(w http.ResponseWriter, r *http.Request) {
	if s.Releases == nil {
		respondJSON(w, http.StatusOK, []platformInfo{})
		return
	}

	available := s.Releases.availablePlatforms()
	if available == nil {
		s.Logger.Warn("no agent builds available")
		available = []platformInfo{}
	}
	respondJSON(w, http.StatusOK, available)
}

// handleProvision generates a one-time registration token and returns
// platform-specific config + install instructions.
//
// POST /api/v1/admin/provision
func (s *Server) handleProvision(w http.ResponseWriter, r *http.Request) {
	var req provisionRequest
	if err := decodeJSONBody(r, &req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Platform == "" {
		http.Error(w, "platform is required", http.StatusBadRequest)
		return
	}

	// Validate platform
	var matched *platformInfo
	if s.Releases != nil {
		for _, p := range s.Releases.availablePlatforms() {
			if p.Filename == req.Platform {
				matched = &p
				break
			}
		}
	}

	if matched == nil {
		// Allow provisioning without a binary, skip download URL
		for i, p := range knownPlatforms {
			if p.Filename == req.Platform {
				matched = &knownPlatforms[i]
				break
			}
		}
	}

	if matched == nil {
		http.Error(w, "unknown platform", http.StatusBadRequest)
		return
	}

	// Generate one-time token
	s.Logger.Info("one-time token provisioned", "ip", clientIP(r))
	token := s.Tokens.Generate(24 * time.Hour)

	// Build server URL from request
	serverURL := s.Config.ExternalURL
	if serverURL == "" {
		scheme := "https"
		if r.TLS == nil {
			scheme = "http"
		}
		serverURL = fmt.Sprintf("%s://%s", scheme, r.Host)
	}

	// Load CA cert if TLS is configured
	var caCertPEM string
	if s.Config.TLSCA != "" {
		data, err := os.ReadFile(s.Config.TLSCA)
		if err != nil {
			s.Logger.Error("failed to read TLS CA", "path", s.Config.TLSCA, "error", err)
			http.Error(w, "TLS CA is configured but could not be read", http.StatusInternalServerError)
			return
		}
		caCertPEM = string(data)
	}
	if !strings.HasSuffix(caCertPEM, "\n") {
		caCertPEM += "\n"
	}

	// If binary exists, build download URL
	downloadURL := ""
	if s.Releases != nil {
		for _, p := range s.Releases.availablePlatforms() {
			if p.Filename == req.Platform {
				downloadURL = fmt.Sprintf("/api/v1/admin/releases/%s", req.Platform)
				break
			}
		}
	}

	caCertPath := ""
	if caCertPEM != "" {
		caCertPath = agentCACertPath(matched.OS)
	}

	config := agentConfig{
		Server: serverURL,
		Token:  token,
		CACert: caCertPath,
	}

	install := generateInstallInstructions(matched, serverURL, token, caCertPEM)

	resp := provisionResponse{
		Token:       token,
		ExpiresAt:   time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		Platform:    req.Platform,
		DownloadURL: downloadURL,
		CACert:      caCertPEM,
		Config:      config,
		Install:     install,
	}

	respondJSON(w, http.StatusCreated, resp)
}

// handleDownloadRelease serves a verified agent binary.
//
// GET /api/v1/admin/releases/{filename}
func (s *Server) handleDownloadRelease(w http.ResponseWriter, r *http.Request) {
	if s.Releases == nil {
		http.Error(w, "releases not configured", http.StatusNotFound)
		return
	}

	filename := r.PathValue("filename")
	if filename == "" || strings.Contains(filename, "/") || strings.Contains(filename, "..") {
		http.Error(w, "invalid filename", http.StatusBadRequest)
		return
	}

	f, size, err := s.Releases.verifyAndOpen(filename)
	if err != nil {
		if strings.Contains(err.Error(), "integrity check failed") {
			s.Logger.Error("binary integrity check failed", "filename", filename, "error", err)
			http.Error(w, "binary integrity check failed", http.StatusInternalServerError)
			return
		}
		http.Error(w, "release not found", http.StatusNotFound)
		return
	}
	defer f.Close()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", size))

	http.ServeContent(w, r, filename, time.Time{}, f)
}

// handleDownloadConfig serves the agent config JSON as a downloadable file.
//
// POST /api/v1/admin/provision/config
func (s *Server) handleDownloadConfig(w http.ResponseWriter, r *http.Request) {
	var cfg agentConfig
	if err := decodeJSONBody(r, &cfg); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="spectra-agent.json"`)

	if _, err := w.Write(data); err != nil {
		s.Logger.Warn("failed to write config response", "error", err)
	}
}

func agentCACertPath(os string) string {
	switch os {
	case "linux":
		return "/etc/spectra/ca.crt"
	case "darwin", "freebsd":
		return "/usr/local/etc/spectra/ca.crt"
	case "windows":
		return `C:\Spectra\ca.crt`
	default:
		return ""
	}
}
