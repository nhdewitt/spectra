package setup

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveAndLoadConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.json")

	cfg := &ServerConfig{
		DatabaseURL: "postgres://user:pass@localhost:5432/spectra?sslmode=disable",
		ListenPort:  8080,
	}

	if err := SaveConfig(cfg, path); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if loaded.DatabaseURL != cfg.DatabaseURL {
		t.Errorf("DatabaseURL = %s, want %s", loaded.DatabaseURL, cfg.DatabaseURL)
	}
	if loaded.ListenPort != cfg.ListenPort {
		t.Errorf("ListenPort = %d, want %d", loaded.ListenPort, cfg.ListenPort)
	}
}

func TestSaveConfig_CreatesDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deep", "server.json")

	cfg := &ServerConfig{DatabaseURL: "test", ListenPort: 8080}
	if err := SaveConfig(cfg, path); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("config file should exist")
	}
}

func TestLoadConfig_NotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path/server.json")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadConfig_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.json")
	os.WriteFile(path, []byte("not json"), 0600)

	_, err := LoadConfig(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestConfigExists_True(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.json")
	os.WriteFile(path, []byte("{}"), 0600)

	if !ConfigExists(path) {
		t.Error("expected true for existing file")
	}
}

func TestConfigExists_False(t *testing.T) {
	if ConfigExists("/nonexistent/path/server.json") {
		t.Error("expected false for missing file")
	}
}

func TestBuildDSN(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		port     string
		dbName   string
		dbUser   string
		dbPass   string
		sslMode  string
		wantHost string
		wantSSL  string
	}{
		{
			name:    "basic",
			host:    "localhost",
			port:    "5432",
			dbName:  "spectra",
			dbUser:  "postgres",
			dbPass:  "secret",
			sslMode: "disable",
		},
		{
			name:    "special chars in password",
			host:    "localhost",
			port:    "5432",
			dbName:  "spectra",
			dbUser:  "postgres",
			dbPass:  "p@ss:w/rd&more=stuff",
			sslMode: "require",
		},
		{
			name:    "special chars in username",
			host:    "localhost",
			port:    "5432",
			dbName:  "spectra",
			dbUser:  "user@domain",
			dbPass:  "pass",
			sslMode: "disable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dsn := buildDSN(tt.host, tt.port, tt.dbName, tt.dbUser, tt.dbPass, tt.sslMode)
			if dsn == "" {
				t.Fatal("buildDSN returned empty string")
			}
			if !strings.Contains(dsn, tt.host) {
				t.Errorf("DSN missing host: %s", dsn)
			}
			if !strings.Contains(dsn, tt.sslMode) {
				t.Errorf("DSN missing sslmode: %s", dsn)
			}
			if !strings.HasPrefix(dsn, "postgres://") {
				t.Errorf("DSN missing scheme: %s", dsn)
			}
		})
	}
}

func TestBuildDSN_PasswordEscaping(t *testing.T) {
	dsn := buildDSN("localhost", "5432", "spectra", "postgres", "p@ss:word", "disable")

	// The @ and : in the password must be escaped so they don't break URL parsing
	if strings.Contains(dsn, "p@ss:word@") {
		t.Errorf("password not escaped in DSN: %s", dsn)
	}
}

func TestPrompt_Default(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("\n"))
	got := prompt(reader, "Test", "default")
	if got != "default" {
		t.Errorf("prompt = %s, want default", got)
	}
}

func TestPrompt_UserInput(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("custom\n"))
	got := prompt(reader, "Test", "default")
	if got != "custom" {
		t.Errorf("prompt = %s, want custom", got)
	}
}

func TestPrompt_TrimWhitespace(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("  spaced  \n"))
	got := prompt(reader, "Test", "")
	if got != "spaced" {
		t.Errorf("prompt = %q, want %q", got, "spaced")
	}
}

func TestPromptRequired_RetriesOnEmpty(t *testing.T) {
	// First line empty, second has value
	reader := bufio.NewReader(strings.NewReader("\n\nactual\n"))
	got := promptRequired(reader, "Test")
	if got != "actual" {
		t.Errorf("promptRequired = %s, want actual", got)
	}
}

func TestFindMigrations(t *testing.T) {
	dir := t.TempDir()

	// Create test migration files
	files := []string{
		"003_third.up.sql",
		"001_first.up.sql",
		"002_second.up.sql",
		"001_first.down.sql", // should be excluded
		"readme.md",          // should be excluded
	}
	for _, f := range files {
		os.WriteFile(filepath.Join(dir, f), []byte("-- test"), 0644)
	}

	got, err := findMigrations(dir)
	if err != nil {
		t.Fatalf("findMigrations: %v", err)
	}

	if len(got) != 3 {
		t.Fatalf("found %d migrations, want 3", len(got))
	}

	// Should be sorted
	want := []string{"001_first.up.sql", "002_second.up.sql", "003_third.up.sql"}
	for i, f := range got {
		if f != want[i] {
			t.Errorf("migration[%d] = %s, want %s", i, f, want[i])
		}
	}
}

func TestFindMigrations_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	got, err := findMigrations(dir)
	if err != nil {
		t.Fatalf("findMigrations: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("found %d migrations, want 0", len(got))
	}
}

func TestFindMigrations_BadDir(t *testing.T) {
	_, err := findMigrations("/nonexistent/path")
	if err == nil {
		t.Error("expected error for missing directory")
	}
}

func TestPromptPort_Valid(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("9090\n"))
	got := PromptPort(reader)
	if got != 9090 {
		t.Errorf("PromptPort = %d, want 9090", got)
	}
}

func TestPromptPort_Default(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("\n"))
	got := PromptPort(reader)
	if got != 8080 {
		t.Errorf("PromptPort = %d, want 8080", got)
	}
}

func TestPromptPort_InvalidThenValid(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("abc\n99999\n4000\n"))
	got := PromptPort(reader)
	if got != 4000 {
		t.Errorf("PromptPort = %d, want 4000", got)
	}
}

func TestPromptPort_Zero(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("0\n8080\n"))
	got := PromptPort(reader)
	if got != 8080 {
		t.Errorf("PromptPort = %d, want 8080", got)
	}
}
