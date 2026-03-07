package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestDefaultConfigPath(t *testing.T) {
	path := DefaultConfigPath()
	if path == "" {
		t.Fatal("expected non-empty config path")
	}

	expected := "/etc/spectra/agent.json"
	switch runtime.GOOS {
	case "windows":
		expected = `C:\spectra\agent.json`
	case "freebsd":
		expected = "/usr/local/etc/spectra/agent.json"
	}

	if path != expected {
		t.Errorf("expected %q, got %q", expected, path)
	}
}

func TestLoadConfig(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name          string
		fileContent   string
		setupFunc     func(path string)
		expectedError bool
		checkConfig   func(*testing.T, *Config)
	}{
		{
			name: "valid config with registration token",
			fileContent: `{
				"server": "https://api.example.com",
				"token": "reg-token-123"
			}`,
			expectedError: false,
			checkConfig: func(t *testing.T, cfg *Config) {
				if cfg.BaseURL != "https://api.example.com" {
					t.Errorf("expected BaseURL https://api.example.com, got %q", cfg.BaseURL)
				}
				if cfg.RegistrationToken != "reg-token-123" {
					t.Errorf("expected RegistrationToken reg-token-123, got %q", cfg.RegistrationToken)
				}
				if cfg.AgentID != "" || cfg.Secret != "" {
					t.Error("expected AgentID and Secret to be empty")
				}
			},
		},
		{
			name: "valid config with agent credentials",
			fileContent: `{
				"server": "https://api.example.com",
				"agent_id": "agent-xyz",
				"secret": "super-secret"
			}`,
			expectedError: false,
			checkConfig: func(t *testing.T, cfg *Config) {
				if cfg.AgentID != "agent-xyz" {
					t.Errorf("expected AgentID agent-xyz, got %q", cfg.AgentID)
				}
				if cfg.Secret != "super-secret" {
					t.Errorf("expected Secret super-secret, got %q", cfg.Secret)
				}
				if cfg.RegistrationToken != "" {
					t.Error("expected RegistrationToken to be empty")
				}
			},
		},
		{
			name:          "file does not exist",
			fileContent:   "", // won't be written
			setupFunc:     func(path string) { os.Remove(path) },
			expectedError: true,
		},
		{
			name:          "invalid json",
			fileContent:   `{"server": "https://api.example.com", bad-json`,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := filepath.Join(tempDir, "agent.json")

			if tt.fileContent != "" {
				err := os.WriteFile(filePath, []byte(tt.fileContent), 0644)
				if err != nil {
					t.Fatalf("failed to write test config: %v", err)
				}
			}

			if tt.setupFunc != nil {
				tt.setupFunc(filePath)
			}

			cfg, err := LoadConfig(filePath)

			if (err != nil) != tt.expectedError {
				t.Fatalf("expected error: %v, got: %v", tt.expectedError, err)
			}

			if err == nil {
				// Check shared defaults
				if cfg.MetricsPath != "/api/v1/agent/metrics" {
					t.Errorf("unexpected MetricsPath: %s", cfg.MetricsPath)
				}
				if cfg.CommandPath != "/api/v1/agent/command" {
					t.Errorf("unexpected CommandPath: %s", cfg.CommandPath)
				}
				if cfg.PollInterval != 5*time.Second {
					t.Errorf("unexpected PollInterval: %v", cfg.PollInterval)
				}
				if cfg.ConfigPath != filePath {
					t.Errorf("unexpected ConfigPath: %s", cfg.ConfigPath)
				}

				// Check case-specific logic
				if tt.checkConfig != nil {
					tt.checkConfig(t, cfg)
				}
			}
		})
	}
}

func TestConfigFromEnv(t *testing.T) {
	// Save the original env var and restore it after the test finishes
	originalEnv, envSet := os.LookupEnv("SPECTRA_SERVER")
	defer func() {
		if envSet {
			os.Setenv("SPECTRA_SERVER", originalEnv)
		} else {
			os.Unsetenv("SPECTRA_SERVER")
		}
	}()

	t.Run("default when env var is empty", func(t *testing.T) {
		os.Unsetenv("SPECTRA_SERVER")
		cfg := ConfigFromEnv()
		if cfg.BaseURL != "http://127.0.0.1:8080" {
			t.Errorf("expected default BaseURL, got %q", cfg.BaseURL)
		}
	})

	t.Run("uses SPECTRA_SERVER env var", func(t *testing.T) {
		os.Setenv("SPECTRA_SERVER", "https://custom.spectra.local")
		cfg := ConfigFromEnv()
		if cfg.BaseURL != "https://custom.spectra.local" {
			t.Errorf("expected BaseURL https://custom.spectra.local, got %q", cfg.BaseURL)
		}
	})
}

func TestSaveCredentials(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test_save.json")

	err := SaveCredentials(filePath, "https://api.example.com", "agent-123", "secret-456")
	if err != nil {
		t.Fatalf("SaveCredentials failed: %v", err)
	}

	// Read it back to verify the contents
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}

	var fc fileConfig
	if err := json.Unmarshal(data, &fc); err != nil {
		t.Fatalf("failed to unmarshal written file: %v", err)
	}

	if fc.Server != "https://api.example.com" {
		t.Errorf("expected Server https://api.example.com, got %q", fc.Server)
	}
	if fc.AgentID != "agent-123" {
		t.Errorf("expected AgentID agent-123, got %q", fc.AgentID)
	}
	if fc.Secret != "secret-456" {
		t.Errorf("expected Secret secret-456, got %q", fc.Secret)
	}
	if fc.Token != "" {
		t.Error("expected Token to be empty")
	}
}
