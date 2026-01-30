package diagnostics

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func TestGetWindowsLevelFlag(t *testing.T) {
	tests := []struct {
		name     string
		level    protocol.LogLevel
		expected string
	}{
		{"emergency", protocol.LevelEmergency, "1"},
		{"alert", protocol.LevelAlert, "1"},
		{"critical", protocol.LevelCritical, "1"},
		{"error", protocol.LevelError, "1,2"},
		{"warning", protocol.LevelWarning, "1,2,3"},
		{"notice", protocol.LevelNotice, "1,2,3,4"},
		{"info", protocol.LevelInfo, "1,2,3,4"},
		{"debug", protocol.LevelDebug, "1,2,3,4"},
		{"unknown defaults to error", protocol.LogLevel("unknown"), "1,2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getWindowsLevelFlag(tt.level)
			if got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestMapWinLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected protocol.LogLevel
	}{
		{"Critical", protocol.LevelCritical},
		{"Error", protocol.LevelError},
		{"Warning", protocol.LevelWarning},
		{"Information", protocol.LevelInfo},
		{"unknown", protocol.LevelInfo},
		{"", protocol.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := mapWinLevel(tt.input)
			if got != tt.expected {
				t.Errorf("got %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestParseWinDate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
	}{
		{
			name:     "valid date",
			input:    "/Date(1736164800000)/",
			expected: 1736164800,
		},
		{
			name:     "zero",
			input:    "/Date(0)/",
			expected: 0,
		},
		{
			name:     "large timestamp",
			input:    "/Date(1893456000000)/",
			expected: 1893456000,
		},
		{
			name:     "invalid format",
			input:    "not a date",
			expected: 0,
		},
		{
			name:     "empty string",
			input:    "",
			expected: 0,
		},
		{
			name:     "missing prefix",
			input:    "1736164800000)/",
			expected: 0,
		},
		{
			name:     "missing suffix",
			input:    "/Date(1736164800000",
			expected: 0,
		},
		{
			name:     "non-numeric",
			input:    "/Date(abc)/",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseWinDate(tt.input)
			if got != tt.expected {
				t.Errorf("got %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestFormatWindowsMessage(t *testing.T) {
	tests := []struct {
		name, input, expected string
	}{
		{
			name:     "simple message",
			input:    "Service started successfully",
			expected: "Service started successfully",
		},
		{
			name:     "message with Context",
			input:    "Error occurred Context: some context info",
			expected: "Error occurred",
		},
		{
			name:     "message with Operation",
			input:    "Failed to start Operation: StartService",
			expected: "Failed to start",
		},
		{
			name:     "multiple whitespace",
			input:    "Error   occurred    here",
			expected: "Error occurred here",
		},
		{
			name:     "newlines and tabs",
			input:    "Line1\n\tLine2\r\nLine3",
			expected: "Line1 Line2 Line3",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only whitespace",
			input:    "   \t\n   ",
			expected: "",
		},
		{
			name:     "Context in middle",
			input:    "Error happened Context: details Operation: more",
			expected: "Error happened",
		},
		{
			name:     "leading/trailing whitespace",
			input:    "  Message with spaces  ",
			expected: "Message with spaces",
		},
		{
			name:     "complex Windows event",
			input:    "The application-specific permission settings do not grant Local Activation permission\r\n\r\nContext:\r\nSource: Microsoft-Windows-Security-Auditing",
			expected: "The application-specific permission settings do not grant Local Activation permission",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatWindowsMessage(tt.input)
			if got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestEncodePowerShell(t *testing.T) {
	tests := []struct {
		name, input string
	}{
		{"simple command", "Get-Process"},
		{"with special chars", `Write-Host "Hello, World!"`},
		{"with newlines", "Line1\nLine2"},
		{"unicode", "こんにちは"},
		{"empty", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded := encodePowerShell(tt.input)

			if encoded != "" && len(encoded)%4 != 0 {
				t.Errorf("invalid base64 length: %d", len(encoded))
			}
			if tt.input != "" && encoded == "" {
				t.Error("expected non-empty encoded output")
			}
		})
	}
}

func TestGetBootTime(t *testing.T) {
	bootTime := getBootTime()
	now := time.Now()

	if bootTime.After(now) {
		t.Errorf("boot time %v is after now %v", bootTime, now)
	}

	oneYearAgo := now.AddDate(-1, 0, 0)
	if bootTime.Before(oneYearAgo) {
		t.Errorf("boot time %v is more than 1 year ago", bootTime)
	}

	bootTime2 := getBootTime()
	diff := bootTime2.Sub(bootTime)
	if diff < 0 {
		diff = -diff
	}
	if diff > time.Second {
		t.Errorf("boot times differ by %v", diff)
	}
}

func TestFetchLogs_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	logs, err := FetchLogs(ctx, protocol.LogRequest{MinLevel: protocol.LevelError})
	if err != nil {
		t.Fatalf("FetchLogs failed: %v", err)
	}

	t.Logf("Fetched %d log entries", len(logs))

	for i, entry := range logs {
		if i > 10 {
			break
		}

		if entry.Source == "" {
			t.Errorf("entry %d: empty source", i)
		}
		if !strings.HasPrefix(entry.Source, "WinEvent:") {
			t.Errorf("entry %d: source should start with 'WinEvent:', got %s", i, entry.Source)
		}
		if entry.Message == "" {
			t.Errorf("entry %d: empty message", i)
		}
		if entry.Level == "" {
			t.Errorf("entry %d: empty level", i)
		}
		if entry.Timestamp == 0 {
			t.Errorf("entry %d: zero timestamp", i)
		}

		t.Logf("  [%d] %s: %s (%.50s...)", i, entry.Level, entry.Source, entry.Message)
	}
}

func TestFetchLogs_AllLevels(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Start with Error as Critical may have no events (causes CI errors)
	levels := []protocol.LogLevel{
		protocol.LevelError,
		protocol.LevelWarning,
		protocol.LevelInfo,
	}

	ctx := context.Background()

	var prevCount int
	for _, level := range levels {
		logs, err := FetchLogs(ctx, protocol.LogRequest{MinLevel: level})
		if err != nil {
			t.Fatalf("FetchLogs(%s) failed: %v", level, err)
		}

		t.Logf("%s level: %d entries", level, len(logs))

		if len(logs) < prevCount {
			t.Errorf("%s returned fewer logs (%d) than previous level (%d)", level, len(logs), prevCount)
		}
		prevCount = len(logs)
	}
}

func TestFetchLogs_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := FetchLogs(ctx, protocol.LogRequest{MinLevel: protocol.LevelError})
	t.Logf("FetchLogs with cancelled context: %v", err)
}

func TestFetchLogs_ContextTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	time.Sleep(1 * time.Millisecond)

	_, err := FetchLogs(ctx, protocol.LogRequest{MinLevel: protocol.LevelError})
	t.Logf("FetchLogs with timeout: %v", err)
}

func TestFetchLogs_RespectsMaxLogs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	logs, err := FetchLogs(ctx, protocol.LogRequest{MinLevel: protocol.LevelInfo})
	if err != nil {
		t.Fatalf("FetchLogs failed: %v", err)
	}
	if len(logs) > MaxLogs {
		t.Errorf("got %d logs, expected at most %d", len(logs), MaxLogs)
	}
}

func TestSpaceCollapserRegex(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"a  b", "a b"},
		{"a   b   c", "a b c"},
		{"a\t\tb", "a b"},
		{"a\n\nb", "a b"},
		{"a \t \n b", "a b"},
		{"no extra spaces", "no extra spaces"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := spaceCollapser.ReplaceAllString(tt.input, " ")
			if got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}

func BenchmarkGetWindowsLevelFlag(b *testing.B) {
	levels := []protocol.LogLevel{
		protocol.LevelCritical,
		protocol.LevelError,
		protocol.LevelWarning,
		protocol.LevelInfo,
	}

	b.ReportAllocs()
	for b.Loop() {
		for _, level := range levels {
			_ = getWindowsLevelFlag(level)
		}
	}
}

func BenchmarkMapWinLevel(b *testing.B) {
	levels := []string{"Critical", "Error", "Warning", "Information", "unknown"}

	b.ReportAllocs()
	for b.Loop() {
		for _, level := range levels {
			_ = mapWinLevel(level)
		}
	}
}

func BenchmarkParseWinDate(b *testing.B) {
	input := "/Date(1736164800000)/"

	b.ReportAllocs()
	for b.Loop() {
		_ = parseWinDate(input)
	}
}

func BenchmarkParseWinDate_Invalid(b *testing.B) {
	input := "not a valid date"

	b.ReportAllocs()
	for b.Loop() {
		_ = parseWinDate(input)
	}
}

func BenchmarkFormatWindowsMessage_Simple(b *testing.B) {
	input := "Service started successfully"

	b.ReportAllocs()
	for b.Loop() {
		_ = formatWindowsMessage(input)
	}
}

func BenchmarkFormatWindowsMessage_Complex(b *testing.B) {
	input := "The application-specific permission settings do not grant Local Activation permission\r\n\r\nContext:\r\nSource: Microsoft-Windows-Security-Auditing\r\nOperation: StartService"

	b.ReportAllocs()
	for b.Loop() {
		_ = formatWindowsMessage(input)
	}
}

func BenchmarkEncodePowerShell_Short(b *testing.B) {
	input := "Get-Process"

	b.ReportAllocs()
	for b.Loop() {
		_ = encodePowerShell(input)
	}
}

func BenchmarkEncodePowerShell_Long(b *testing.B) {
	input := `[Console]::OutputEncoding = [System.Text.Encoding]::UTF8;
		$query = "*[System[(Level=1 or Level=2) and TimeCreated[@SystemTime>='2025-01-06T00:00:00Z']]]";
		Get-WinEvent -LogName @('System','Application') -FilterXPath $query -MaxEvents 10000 -ErrorAction SilentlyContinue -Oldest |
		Select-Object TimeCreated, LevelDisplayName, Message, ProviderName, ProcessId |
		ForEach-Object { $_ | ConvertTo-Json -Compress }`

	b.ReportAllocs()
	for b.Loop() {
		_ = encodePowerShell(input)
	}
}

func BenchmarkGetBootTime(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = getBootTime()
	}
}

func BenchmarkSpaceCollapser(b *testing.B) {
	input := "Error   occurred\r\n\twith   multiple    spaces\r\n\r\nand newlines"

	b.ReportAllocs()
	for b.Loop() {
		_ = spaceCollapser.ReplaceAllString(input, " ")
	}
}

func BenchmarkFetchLogs_Error(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping integration benchmark")
	}

	ctx := context.Background()

	logs, err := FetchLogs(ctx, protocol.LogRequest{MinLevel: protocol.LevelError})
	if err != nil {
		b.Skipf("FetchLogs not available: %v", err)
	}
	b.Logf("Error level: %d entries", len(logs))

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = FetchLogs(ctx, protocol.LogRequest{MinLevel: protocol.LevelError})
	}
}

func BenchmarkFetchLogs_Warning(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping integration benchmark")
	}

	ctx := context.Background()

	logs, err := FetchLogs(ctx, protocol.LogRequest{MinLevel: protocol.LevelWarning})
	if err != nil {
		b.Skipf("FetchLogs not available: %v", err)
	}
	b.Logf("Warning level: %d entries", len(logs))

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = FetchLogs(ctx, protocol.LogRequest{MinLevel: protocol.LevelWarning})
	}
}

func BenchmarkFetchLogs_Info(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping integration benchmark")
	}

	ctx := context.Background()

	logs, err := FetchLogs(ctx, protocol.LogRequest{MinLevel: protocol.LevelInfo})
	if err != nil {
		b.Skipf("FetchLogs not available: %v", err)
	}
	b.Logf("Info level: %d entries", len(logs))

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = FetchLogs(ctx, protocol.LogRequest{MinLevel: protocol.LevelInfo})
	}
}
