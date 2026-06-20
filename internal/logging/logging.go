package logging

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/natefinch/lumberjack.v2"
)

// defaultLogDir returns the appropriate log directory.
//
//   - Linux/macOS/FreeBSD: /var/log/spectra
//   - Windows: %ProgramData%\Spectra\logs
func defaultLogDir() string {
	if runtime.GOOS == "windows" {
		base := os.Getenv("ProgramData")
		if base == "" {
			base = `C:\ProgramData`
		}
		return filepath.Join(base, "Spectra", "logs")
	}
	return "/var/log/spectra"
}

// Config controls how logging is set up.
type Config struct {
	FilePath     string
	ConsoleLevel slog.Level
	FileLevel    slog.Level
	MaxSizeMB    int
	MaxBackups   int
	MaxAgeDays   int
	Compress     bool
}

// DefaultServerConfig returns defaults for the Spectra server.
func DefaultServerConfig() Config {
	return Config{
		FilePath:     filepath.Join(defaultLogDir(), "server.log"),
		ConsoleLevel: slog.LevelInfo,
		FileLevel:    slog.LevelDebug,
		MaxSizeMB:    50,
		MaxBackups:   3,
		MaxAgeDays:   30,
		Compress:     true,
	}
}

// DefaultAgentConfig returns defaults for the Spectra agent.
func DefaultAgentConfig() Config {
	return Config{
		FilePath:     filepath.Join(defaultLogDir(), "agent.log"),
		ConsoleLevel: slog.LevelInfo,
		FileLevel:    slog.LevelDebug,
		MaxSizeMB:    10,
		MaxBackups:   2,
		MaxAgeDays:   14,
		Compress:     true,
	}
}

// Logger wraps slog.Logger and exposes a LevelVar for runtime level changes.
type Logger struct {
	*slog.Logger
	ConsoleLevel *slog.LevelVar
	FileLevel    *slog.LevelVar
	fileWriter   io.WriteCloser
}

// New creates a Logger from the given config. The returned Logger should have
// Close called on shutdown to flush the log file.
func New(cfg Config) *Logger {
	consoleLevel := &slog.LevelVar{}
	consoleLevel.Set(cfg.ConsoleLevel)

	fileLevel := &slog.LevelVar{}
	fileLevel.Set(cfg.FileLevel)

	consoleHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: consoleLevel,
	})

	if cfg.FilePath == "" {
		return &Logger{
			Logger:       slog.New(consoleHandler),
			ConsoleLevel: consoleLevel,
			FileLevel:    fileLevel,
		}
	}

	lj := &lumberjack.Logger{
		Filename:   cfg.FilePath,
		MaxSize:    cfg.MaxSizeMB,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAgeDays,
		Compress:   cfg.Compress,
	}

	fileHandler := slog.NewJSONHandler(lj, &slog.HandlerOptions{
		Level: fileLevel,
	})

	multi := slog.NewMultiHandler(consoleHandler, fileHandler)

	return &Logger{
		Logger:       slog.New(multi),
		ConsoleLevel: consoleLevel,
		FileLevel:    fileLevel,
		fileWriter:   lj,
	}
}

// Close flushes and closes the log file writer. Safe to call if file logging is disabled.
func (l *Logger) Close() error {
	if l.fileWriter != nil {
		return l.fileWriter.Close()
	}
	return nil
}

// SetConsoleLevel changes the console output level at runtime.
func (l *Logger) SetConsoleLevel(level slog.Level) {
	l.ConsoleLevel.Set(level)
}

// SetFileLevel changes the file output level at runtime.
func (l *Logger) SetFileLevel(level slog.Level) {
	l.FileLevel.Set(level)
}

// ParseLevel converts a string to a slog.Level (default: slog.LevelInfo)
func ParseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "info", "":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// NewDiscard returns a Logger that discards all output. For tests and benchmarks.
func NewDiscard() *Logger {
	level := &slog.LevelVar{}
	level.Set(slog.LevelError)
	return &Logger{
		Logger:       slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: level})),
		ConsoleLevel: level,
		FileLevel:    level,
	}
}
