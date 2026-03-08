package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
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
	Config      agentConfig         `json:"config"`
	Install     installInstructions `json:"install"`
}

type agentConfig struct {
	Server   string `json:"server"`
	Token    string `json:"token"`
	Interval int    `json:"interval"`
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
	token := s.Tokens.Generate(24 * time.Hour)

	// Build server URL from request
	scheme := "https"
	if r.TLS == nil {
		scheme = "http"
	}
	serverURL := fmt.Sprintf("%s://%s", scheme, r.Host)

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

	config := agentConfig{
		Server: serverURL,
		Token:  token,
	}

	install := generateInstallInstructions(matched, serverURL, token)

	resp := provisionResponse{
		Token:       token,
		ExpiresAt:   time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		Platform:    req.Platform,
		DownloadURL: downloadURL,
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
		log.Printf("Failed to write config response: %v", err)
	}
}

// Installation instruction generators

func generateInstallInstructions(p *platformInfo, serverURL, token string) installInstructions {
	switch p.OS {
	case "linux":
		return generateSystemdInstructions(p, serverURL, token)
	case "darwin":
		return generateLaunchdInstructions(p, serverURL, token)
	case "freebsd":
		return generateRCDInstructions(p, serverURL, token)
	case "windows":
		return generateWindowsInstructions(p, serverURL, token)
	default:
		return installInstructions{
			Type:    "manual",
			Content: "",
			Steps:   fmt.Sprintf("./%s -register -server %s -token %s", p.Filename, serverURL, token),
		}
	}
}

func generateSystemdInstructions(p *platformInfo, serverURL, token string) installInstructions {
	unit := `[Unit]
Description=Spectra Monitoring Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/spectra-agent
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
`

	steps := fmt.Sprintf(`# 1. Download and install the binary
sudo cp %s /usr/local/bin/spectra-agent
sudo chmod +x /usr/local/bin/spectra-agent

# Place the config file
sudo mkdir -p /etc/spectra
sudo cp spectra-agent.json /etc/spectra/agent.json

# Install the systemd service
sudo tee /etc/systemd/system/spectra-agent.service << 'EOF'
%sEOF

# Enable and start
sudo systemctl daemon-reload
sudo systemctl enable --now spectra-agent

# Verify
sudo systemctl status spectra-agent
`, p.Filename, unit)

	return installInstructions{
		Type:    "systemd",
		Content: unit,
		Steps:   steps,
	}
}

func generateLaunchdInstructions(p *platformInfo, serverURL, token string) installInstructions {
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd"
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>com.spectra.agent</string>
	<key>ProgramArguments</key>
	<array>
		<string>/usr/local/bin/spectra-agent</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
	<key>StandardOutPath</key>
	<string>/var/log/spectra-agent.log</string>
	<key>StandardErrorPath</key>
	<string>/var/log/spectra-agent.err</string>
</dict>
</plist>
`

	steps := fmt.Sprintf(`# Download and install the binary
sudo cp %s /usr/local/bin/spectra-agent
sudo chmod +x /usr/local/bin/spectra-agent

# Place config file
sudo mkdir -p /etc/spectra
sudo cp spectra-agent.json /etc/spectra/agent.json

# Install the launchd service
sudo tee /Library/LaunchDaemons/com.spectra.agent.plist << 'EOF'
%sEOF

# Load and start
sudo launchctl load /Library/LaunchDaemons/com.spectra.agent.plist

# Verify
sudo launchctl list | grep spectra
`, p.Filename, plist)

	return installInstructions{
		Type:    "launchd",
		Content: plist,
		Steps:   steps,
	}
}

func generateRCDInstructions(p *platformInfo, serverURL, token string) installInstructions {
	rcScript := `#!/bin/sh
	
# PROVIDE: spectra_agent
# REQUIRE: NETWORKING
# KEYWORD: shutdown

. /etc/rc.subr

name="spectra_agent"
rcvar="${name}_enable"

command="/usr/local/bin/spectra-agent"
command_args=""

load_rc_config $name
run_rc_command "$1"
`

	steps := fmt.Sprintf(`# 1. Download and install the binary
sudo cp %s /usr/local/bin/spectra-agent
sudo chmod +x /usr/local/bin/spectra-agent

# 2. Register the agent (one-time)
sudo mkdir -p /usr/local/etc/spectra
sudo cp spectra-agent.json /usr/local/etc/spectra/agent.json

# 3. Install the rc.d script
sudo tee /usr/local/etc/rc.d/spectra_agent << 'EOF'
%sEOF
sudo chmod +x /usr/local/etc/rc.d/spectra_agent

# 4. Enable and start
echo 'spectra_agent_enable="YES"' | sudo tee -a /etc/rc.conf
sudo service spectra_agent start

# 5. Verify
sudo service spectra_agent status
`, p.Filename, rcScript)

	return installInstructions{
		Type:    "rc_d",
		Content: rcScript,
		Steps:   steps,
	}
}

func generateWindowsInstructions(p *platformInfo, serverURL, token string) installInstructions {
	steps := fmt.Sprintf(`# Download the binary to a permanent location
New-Item -Path "C:\spectra" -ItemType Directory
Move-Item %s C:\spectra\spectra-agent.exe

# Place config file
Copy-Item spectra-agent.json C:\spectra\agent.json

# Install as a Windows service (run as Administrator)
sc.exe create SpectraAgent binPath= "C:\spectra\spectra-agent.exe" start= auto
sc.exe description SpectraAgent "Spectra Monitoring Agent"

# Start the service
sc.exe start SpectraAgent

# Verify
sc.exe query SpectraAgent
`, p.Filename)

	return installInstructions{
		Type:    "windows_service",
		Content: "",
		Steps:   steps,
	}
}
