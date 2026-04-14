package logging

import (
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDefaultLogDir(t *testing.T) {
	dir := defaultLogDir()

	if runtime.GOOS == "windows" {
		if dir == "/var/log/spectra" {
			t.Error("Windows should not use /var/log/spectra")
		}
		if !filepath.IsAbs(dir) {
			t.Errorf("expected absolute path, got %q", dir)
		}
	} else {
		if dir != "/var/log/spectra" {
			t.Errorf("expected /var/log/spectra, got %q", dir)
		}
	}
}

func TestDefaultServerConfig(t *testing.T) {
	cfg := DefaultServerConfig()

	if cfg.ConsoleLevel != slog.LevelInfo {
		t.Errorf("expected ConsoleLevel = Info, got %v", cfg.ConsoleLevel)
	}
	if cfg.FileLevel != slog.LevelDebug {
		t.Errorf("expected FileLevel = Debug, got %v", cfg.FileLevel)
	}
	if cfg.MaxSizeMB != 50 {
		t.Errorf("expected MaxSizeMB = 50, got %d", cfg.MaxSizeMB)
	}
	if cfg.MaxBackups != 3 {
		t.Errorf("expected MaxBackups = 3, got %d", cfg.MaxBackups)
	}
	if cfg.MaxAgeDays != 30 {
		t.Errorf("expected MaxAgeDays = 30, got %d", cfg.MaxAgeDays)
	}
	if !cfg.Compress {
		t.Error("expected Compress = true")
	}
	if cfg.FilePath == "" {
		t.Error("expected non-empty FilePath")
	}
}

func TestDefaultAgentConfig(t *testing.T) {
	cfg := DefaultAgentConfig()

	if cfg.ConsoleLevel != slog.LevelInfo {
		t.Errorf("expected ConsoleLevel = Info, got %v", cfg.ConsoleLevel)
	}
	if cfg.FileLevel != slog.LevelDebug {
		t.Errorf("expected FileLevel = Debug, got %v", cfg.FileLevel)
	}
	if cfg.MaxSizeMB != 10 {
		t.Errorf("expected MaxSizeMB = 10, got %d", cfg.MaxSizeMB)
	}
	if cfg.MaxBackups != 2 {
		t.Errorf("expected MaxBackups = 2, got %d", cfg.MaxBackups)
	}
	if cfg.MaxAgeDays != 14 {
		t.Errorf("expected MaxAgeDays = 14, got %d", cfg.MaxAgeDays)
	}
	if !cfg.Compress {
		t.Error("expected Compress = true")
	}
}

func TestDefaultConfigs_DifferentLimits(t *testing.T) {
	server := DefaultServerConfig()
	agent := DefaultAgentConfig()

	if server.MaxSizeMB <= agent.MaxSizeMB {
		t.Error("server MaxSizeMB should be larger than agent")
	}
	if server.MaxBackups <= agent.MaxBackups {
		t.Error("server MaxBackups should be larger than agent")
	}
	if server.MaxAgeDays <= agent.MaxAgeDays {
		t.Error("server MaxAgeDays should be larger than agent")
	}
}

func TestNew_ConsoleOnly(t *testing.T) {
	cfg := Config{
		FilePath:     "",
		ConsoleLevel: slog.LevelWarn,
	}

	logger := New(cfg)
	defer logger.Close()

	if logger.Logger == nil {
		t.Fatal("expected non-nil Logger")
	}
	if logger.ConsoleLevel == nil {
		t.Fatal("expected non-nil ConsoleLevel")
	}
	if logger.FileLevel == nil {
		t.Fatal("expected non-nil FileLevel")
	}
	if logger.fileWriter != nil {
		t.Error("expected nil fileWriter for console-only mode")
	}
}

func TestNew_WithFile(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	cfg := Config{
		FilePath:     logPath,
		ConsoleLevel: slog.LevelInfo,
		FileLevel:    slog.LevelDebug,
		MaxSizeMB:    1,
		MaxBackups:   1,
		MaxAgeDays:   1,
		Compress:     false,
	}

	logger := New(cfg)
	defer logger.Close()

	if logger.Logger == nil {
		t.Fatal("expected non-nil Logger")
	}
	if logger.fileWriter == nil {
		t.Error("expected non-nil fileWriter")
	}

	logger.Info("test message", "key", "value")

	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("expected log file to be created")
	}
}

func TestNew_WritesToFile(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	cfg := Config{
		FilePath:     logPath,
		ConsoleLevel: slog.LevelInfo,
		FileLevel:    slog.LevelDebug,
		MaxSizeMB:    1,
		MaxBackups:   1,
		MaxAgeDays:   1,
	}

	logger := New(cfg)
	logger.Info("hello from test")
	logger.Close()

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	content := string(data)
	if len(content) == 0 {
		t.Error("expected non-empty log file")
	}
	if content[0] != '{' {
		t.Errorf("expected JSON output, got: %s", content[:min(50, len(content))])
	}
}

func TestClose_NilFileWriter(t *testing.T) {
	logger := &Logger{}
	if err := logger.Close(); err != nil {
		t.Errorf("Close() on nul fileWriter should not error, got: %v", err)
	}
}

func TestClose_WithFileWriter(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	logger := New(Config{
		FilePath:  logPath,
		MaxSizeMB: 1,
	})

	if err := logger.Close(); err != nil {
		t.Errorf("Close() should not error, got: %v", err)
	}
}

func TestSetConsoleLevel(t *testing.T) {
	logger := New(Config{ConsoleLevel: slog.LevelInfo})
	defer logger.Close()

	if logger.ConsoleLevel.Level() != slog.LevelInfo {
		t.Fatalf("expected initial level Info, got %v", logger.ConsoleLevel.Level())
	}

	logger.SetConsoleLevel(slog.LevelDebug)
	if logger.ConsoleLevel.Level() != slog.LevelDebug {
		t.Errorf("expected Debug after SetConsoleLevel, got %v", logger.ConsoleLevel.Level())
	}

	logger.SetConsoleLevel(slog.LevelError)
	if logger.ConsoleLevel.Level() != slog.LevelError {
		t.Errorf("expected Error after SetConsoleLevel, got %v", logger.ConsoleLevel.Level())
	}
}

func TestSetFileLevel(t *testing.T) {
	logger := New(Config{FileLevel: slog.LevelDebug})
	defer logger.Close()

	if logger.FileLevel.Level() != slog.LevelDebug {
		t.Fatalf("expected initial level Debug, got %v", logger.FileLevel.Level())
	}

	logger.SetFileLevel(slog.LevelWarn)
	if logger.FileLevel.Level() != slog.LevelWarn {
		t.Errorf("expected Warn after SetFileLevel, got %v", logger.FileLevel.Level())
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"INFO", slog.LevelInfo},
		{"", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"WARN", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"WARNING", slog.LevelWarn},
		{"error", slog.LevelError},
		{"ERROR", slog.LevelError},
		{"garbage", slog.LevelInfo},
		{"TRACE", slog.LevelInfo},
		{"fatal", slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := ParseLevel(tt.input); got != tt.want {
				t.Errorf("ParseLevel(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestFileLevelFiltering(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	logger := New(Config{
		FilePath:     logPath,
		ConsoleLevel: slog.LevelError, // suppress console
		FileLevel:    slog.LevelWarn,  // only warn+ to file
		MaxSizeMB:    1,
	})

	logger.Debug("should not appear")
	logger.Info("should not appear either")
	logger.Warn("should appear")
	logger.Close()

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	content := string(data)
	if content == "" {
		t.Fatal("expected non-empty log file")
	}

	// Should contain the warn message but not debug/info
	if !contains(content, "should appear") {
		t.Error("expected warn message in log file")
	}
	if contains(content, "should not appear") {
		t.Error("debug/info messages should be filtered out")
	}
}

func TestRuntimeLevelChange(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	logger := New(Config{
		FilePath:     logPath,
		ConsoleLevel: slog.LevelError,
		FileLevel:    slog.LevelError, // start strict
		MaxSizeMB:    1,
	})

	logger.Info("before level change")

	// Loosen file level
	logger.SetFileLevel(slog.LevelInfo)

	logger.Info("after level change")
	logger.Close()

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}

	content := string(data)
	if contains(content, "before level change") {
		t.Error("message before level change should be filtered")
	}
	if !contains(content, "after level change") {
		t.Error("message after level change should appear")
	}
}

func TestDefaultLogDir_PlatformSpecific(t *testing.T) {
	dir := defaultLogDir()

	switch runtime.GOOS {
	case "windows":
		if dir == "/var/log/spectra" {
			t.Error("Windows should not use Unix log path")
		}
		// Should contain Spectra
		if !contains(dir, "Spectra") {
			t.Errorf("Windows path should contain 'Spectra', got %q", dir)
		}
	default:
		if dir != "/var/log/spectra" {
			t.Errorf("Unix systems should use /var/log/spectra, got %q", dir)
		}
	}
}

func TestServerAndAgentPaths_Differ(t *testing.T) {
	server := DefaultServerConfig()
	agent := DefaultAgentConfig()

	if server.FilePath == agent.FilePath {
		t.Error("server and agent should have different log file paths")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
