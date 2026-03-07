package agent

import (
	"encoding/json"
	"os"
	"runtime"
	"time"

	"github.com/nhdewitt/spectra/internal/fileutil"
)

// fileConfig represents the JSON config file on disk.
// After registration, Token is cleared and AgentID/Secret are written.
type fileConfig struct {
	Server  string `json:"server"`
	Token   string `json:"token,omitempty"`
	AgentID string `json:"agent_id,omitempty"`
	Secret  string `json:"secret,omitempty"`
}

// DefaultConfigPath returns the OS-appropriate config file location.
func DefaultConfigPath() string {
	switch runtime.GOOS {
	case "windows":
		return `C:\spectra\agent.json`
	case "freebsd":
		return "/usr/local/etc/spectra/agent.json"
	default:
		return "/etc/spectra/agent.json"
	}
}

// LoadConfig reads and parses the config file, returning a Config
// ready for agent.New(). If the agent is already registered, the
// agent_id + secret are used. If only a token is present, the agent
// will register on first start.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var fc fileConfig
	if err := json.Unmarshal(data, &fc); err != nil {
		return nil, err
	}

	cfg := &Config{
		BaseURL:      fc.Server,
		MetricsPath:  "/api/v1/agent/metrics",
		CommandPath:  "/api/v1/agent/command",
		PollInterval: 5 * time.Second,
		ConfigPath:   path,
	}

	if fc.AgentID != "" && fc.Secret != "" {
		cfg.AgentID = fc.AgentID
		cfg.Secret = fc.Secret
	} else if fc.Token != "" {
		cfg.RegistrationToken = fc.Token
	}

	return cfg, nil
}

// ConfigFromEnv builds a Config from environment variables.
// Backwards-compatible with the original env-based setup.
func ConfigFromEnv() *Config {
	baseURL := os.Getenv("SPECTRA_SERVER")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8080"
	}

	return &Config{
		BaseURL:      baseURL,
		MetricsPath:  "/api/v1/agent/metrics",
		CommandPath:  "/api/v1/agent/command",
		PollInterval: 5 * time.Second,
	}
}

// SaveCredentials writes the permanent agent_id+secret back to
// the config file after registration and clears the one-time
// token.
func SaveCredentials(path, server, agentID, secret string) error {
	fc := fileConfig{
		Server:  server,
		AgentID: agentID,
		Secret:  secret,
	}

	data, err := json.MarshalIndent(fc, "", "  ")
	if err != nil {
		return err
	}

	return fileutil.WriteSecure(path, data)
}
