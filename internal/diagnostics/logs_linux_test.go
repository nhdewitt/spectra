//go:build !windows

package diagnostics

import (
	"context"
	"strings"
	"testing"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func TestBuildDmesgLevelFlag(t *testing.T) {
	tests := []struct {
		name     string
		level    protocol.LogLevel
		expected string
	}{
		{"debug includes all", protocol.LevelDebug, "debug,info,notice,warn,err,crit,alert,emerg"},
		{"info and above", protocol.LevelInfo, "info,notice,warn,err,crit,alert,emerg"},
		{"notice and above", protocol.LevelNotice, "notice,warn,err,crit,alert,emerg"},
		{"warning and above", protocol.LevelWarning, "warn,err,crit,alert,emerg"},
		{"error and above", protocol.LevelError, "err,crit,alert,emerg"},
		{"critical and above", protocol.LevelCritical, "crit,alert,emerg"},
		{"alert and above", protocol.LevelAlert, "alert,emerg"},
		{"emergency only", protocol.LevelEmergency, "emerg"},
		{"unknown defaults to error", protocol.LogLevel("unknown"), "err,crit,alert,emerg"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildDmesgLevelFlag(tt.level)
			if got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestParseDmesgLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected protocol.LogLevel
	}{
		{"emerg", protocol.LevelEmergency},
		{"alert", protocol.LevelAlert},
		{"crit", protocol.LevelCritical},
		{"err", protocol.LevelError},
		{"warn", protocol.LevelWarning},
		{"notice", protocol.LevelNotice},
		{"info", protocol.LevelInfo},
		{"debug", protocol.LevelDebug},
		{"unknown", protocol.LevelInfo},
		{"", protocol.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseDmesgLevel(tt.input)
			if got != tt.expected {
				t.Errorf("got %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestParseDmesgTimestampAndMsg(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectedTS  int64
		expectedMsg string
	}{
		{
			name:        "valid timestamp and message",
			input:       "[Mon Jan  6 12:00:00 2025] Some kernel message here",
			expectedTS:  1736164800,
			expectedMsg: "Some kernel message here",
		},
		{
			name:        "no brackets",
			input:       "Message without timestamp",
			expectedTS:  0,
			expectedMsg: "Message without timestamp",
		},
		{
			name:        "empty message after timestamp",
			input:       "[Mon Jan  6 12:00:00 2025]",
			expectedTS:  1736164800,
			expectedMsg: "",
		},
		{
			name:        "malformed timestamp",
			input:       "[not a real timestamp] Some message",
			expectedTS:  0,
			expectedMsg: "Some message",
		},
		{
			name:        "empty input",
			input:       "",
			expectedTS:  0,
			expectedMsg: "",
		},
		{
			name:        "only opening bracket",
			input:       "[incomplete",
			expectedTS:  0,
			expectedMsg: "[incomplete",
		},
		{
			name:        "message with special characters",
			input:       "[Mon Jan  6 12:00:00 2025] usb 1-1: new device [USB]",
			expectedTS:  1736164800,
			expectedMsg: "usb 1-1: new device [USB]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts, msg := parseDmesgTimestampAndMsg(tt.input)
			if ts != tt.expectedTS {
				t.Errorf("timestamp: got %d, want %d", ts, tt.expectedTS)
			}
			if msg != tt.expectedMsg {
				t.Errorf("message: got %q, want %q", msg, tt.expectedMsg)
			}
		})
	}
}

func TestParseDmesgFrom(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []protocol.LogEntry
	}{
		{
			name:  "single kernel entry",
			input: `kern  :warn  : [Mon Jan  6 12:00:00 2025] CPU0: Core temperature above threshold`,
			expected: []protocol.LogEntry{
				{
					Timestamp: 1736164800,
					Source:    "dmesg:kernel",
					Level:     protocol.LevelWarning,
					Message:   "CPU0: Core temperature above threshold",
				},
			},
		},
		{
			name: "multiple entries",
			input: `kern  :info  : [Mon Jan  6 12:00:00 2025] Initializing subsystem
user  :err   : [Mon Jan  6 12:00:01 2025] Application error occurred`,
			expected: []protocol.LogEntry{
				{
					Timestamp: 1736164800,
					Source:    "dmesg:kernel",
					Level:     protocol.LevelInfo,
					Message:   "Initializing subsystem",
				},
				{
					Timestamp: 1736164801,
					Source:    "dmesg:user",
					Level:     protocol.LevelError,
					Message:   "Application error occurred",
				},
			},
		},
		{
			name: "timestamp fallback to previous",
			input: `kern  :info  : [Mon Jan  6 12:00:00 2025] First message
kern  :info  : [invalid timestamp] Second message`,
			expected: []protocol.LogEntry{
				{
					Timestamp: 1736164800,
					Source:    "dmesg:kernel",
					Level:     protocol.LevelInfo,
					Message:   "First message",
				},
				{
					Timestamp: 1736164800,
					Source:    "dmesg:kernel",
					Level:     protocol.LevelInfo,
					Message:   "Second message",
				},
			},
		},
		{
			name:     "empty input",
			input:    "",
			expected: nil,
		},
		{
			name:     "malformed line - not enough parts",
			input:    "this is not valid dmesg output",
			expected: nil,
		},
		{
			name:     "empty message filtered",
			input:    `kern  :info  : [Mon Jan  6 12:00:00 2025]`,
			expected: nil,
		},
		{
			name: "various facilities",
			input: `kern  :info  : [Mon Jan  6 12:00:00 2025] Kernel msg
user  :info  : [Mon Jan  6 12:00:00 2025] User msg
daemon:info  : [Mon Jan  6 12:00:00 2025] Daemon msg`,
			expected: []protocol.LogEntry{
				{Timestamp: 1736164800, Source: "dmesg:kernel", Level: protocol.LevelInfo, Message: "Kernel msg"},
				{Timestamp: 1736164800, Source: "dmesg:user", Level: protocol.LevelInfo, Message: "User msg"},
				{Timestamp: 1736164800, Source: "dmesg:daemon", Level: protocol.LevelInfo, Message: "Daemon msg"},
			},
		},
		{
			name: "all severity levels",
			input: `kern  :emerg : [Mon Jan  6 12:00:00 2025] Emergency
kern  :alert : [Mon Jan  6 12:00:00 2025] Alert
kern  :crit  : [Mon Jan  6 12:00:00 2025] Critical
kern  :err   : [Mon Jan  6 12:00:00 2025] Error
kern  :warn  : [Mon Jan  6 12:00:00 2025] Warning
kern  :notice: [Mon Jan  6 12:00:00 2025] Notice
kern  :info  : [Mon Jan  6 12:00:00 2025] Info
kern  :debug : [Mon Jan  6 12:00:00 2025] Debug`,
			expected: []protocol.LogEntry{
				{Timestamp: 1736164800, Source: "dmesg:kernel", Level: protocol.LevelEmergency, Message: "Emergency"},
				{Timestamp: 1736164800, Source: "dmesg:kernel", Level: protocol.LevelAlert, Message: "Alert"},
				{Timestamp: 1736164800, Source: "dmesg:kernel", Level: protocol.LevelCritical, Message: "Critical"},
				{Timestamp: 1736164800, Source: "dmesg:kernel", Level: protocol.LevelError, Message: "Error"},
				{Timestamp: 1736164800, Source: "dmesg:kernel", Level: protocol.LevelWarning, Message: "Warning"},
				{Timestamp: 1736164800, Source: "dmesg:kernel", Level: protocol.LevelNotice, Message: "Notice"},
				{Timestamp: 1736164800, Source: "dmesg:kernel", Level: protocol.LevelInfo, Message: "Info"},
				{Timestamp: 1736164800, Source: "dmesg:kernel", Level: protocol.LevelDebug, Message: "Debug"},
			},
		},
		{
			name: "mixed valid and invalid lines",
			input: `kern  :info  : [Mon Jan  6 12:00:00 2025] Valid message
invalid line without colons
kern  :warn  : [Mon Jan  6 12:00:01 2025] Another valid message`,
			expected: []protocol.LogEntry{
				{Timestamp: 1736164800, Source: "dmesg:kernel", Level: protocol.LevelInfo, Message: "Valid message"},
				{Timestamp: 1736164801, Source: "dmesg:kernel", Level: protocol.LevelWarning, Message: "Another valid message"},
			},
		},
		{
			name:     "only whitespace",
			input:    "   \n   \n   ",
			expected: nil,
		},
		{
			name:  "message with colons",
			input: `kern  :info  : [Mon Jan  6 12:00:00 2025] usb 1-1: Device: vendor=0x1234: product=0x5678`,
			expected: []protocol.LogEntry{
				{Timestamp: 1736164800, Source: "dmesg:kernel", Level: protocol.LevelInfo, Message: "usb 1-1: Device: vendor=0x1234: product=0x5678"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDmesgFrom(strings.NewReader(tt.input), 10000)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != len(tt.expected) {
				t.Fatalf("length mismatch: got %d, want %d", len(got), len(tt.expected))
			}

			for i := range got {
				if got[i].Timestamp != tt.expected[i].Timestamp {
					t.Errorf("[%d] timestamp: got %d, want %d", i, got[i].Timestamp, tt.expected[i].Timestamp)
				}
				if got[i].Source != tt.expected[i].Source {
					t.Errorf("[%d] source: got %q, want %q", i, got[i].Source, tt.expected[i].Source)
				}
				if got[i].Level != tt.expected[i].Level {
					t.Errorf("[%d] level: got %v, want %v", i, got[i].Level, tt.expected[i].Level)
				}
				if got[i].Message != tt.expected[i].Message {
					t.Errorf("[%d] message: got %q, want %q", i, got[i].Message, tt.expected[i].Message)
				}
			}
		})
	}
}

func TestParseJournalFrom(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []protocol.LogEntry
	}{
		{
			name:  "single entry with systemd unit",
			input: `{"MESSAGE":"Service started","_SYSTEMD_UNIT":"nginx.service","PRIORITY":"6","__REALTIME_TIMESTAMP":"1736164800000000","_COMM":"nginx","_PID":"1234"}`,
			expected: []protocol.LogEntry{
				{
					Timestamp:   1736164800,
					Source:      "journald:nginx.service",
					Level:       protocol.LevelInfo,
					Message:     "Service started",
					ProcessName: "nginx",
					ProcessID:   1234,
				},
			},
		},
		{
			name:  "fallback to syslog identifier",
			input: `{"MESSAGE":"Log message","SYSLOG_IDENTIFIER":"myapp","PRIORITY":"4","__REALTIME_TIMESTAMP":"1736164800000000"}`,
			expected: []protocol.LogEntry{
				{
					Timestamp: 1736164800,
					Source:    "journald:myapp",
					Level:     protocol.LevelWarning,
					Message:   "Log message",
				},
			},
		},
		{
			name:  "fallback to comm",
			input: `{"MESSAGE":"Process output","_COMM":"bash","PRIORITY":"6","__REALTIME_TIMESTAMP":"1736164800000000"}`,
			expected: []protocol.LogEntry{
				{
					Timestamp:   1736164800,
					Source:      "journald:bash",
					Level:       protocol.LevelInfo,
					Message:     "Process output",
					ProcessName: "bash",
				},
			},
		},
		{
			name:  "fallback to unknown",
			input: `{"MESSAGE":"Mystery message","PRIORITY":"6","__REALTIME_TIMESTAMP":"1736164800000000"}`,
			expected: []protocol.LogEntry{
				{
					Timestamp: 1736164800,
					Source:    "journald:unknown",
					Level:     protocol.LevelInfo,
					Message:   "Mystery message",
				},
			},
		},
		{
			name: "multiple entries",
			input: `{"MESSAGE":"First","_SYSTEMD_UNIT":"a.service","PRIORITY":"6","__REALTIME_TIMESTAMP":"1736164800000000"}
{"MESSAGE":"Second","_SYSTEMD_UNIT":"b.service","PRIORITY":"3","__REALTIME_TIMESTAMP":"1736164801000000"}`,
			expected: []protocol.LogEntry{
				{Timestamp: 1736164800, Source: "journald:a.service", Level: protocol.LevelInfo, Message: "First"},
				{Timestamp: 1736164801, Source: "journald:b.service", Level: protocol.LevelError, Message: "Second"},
			},
		},
		{
			name:     "empty message filtered",
			input:    `{"MESSAGE":"","_SYSTEMD_UNIT":"test.service","PRIORITY":"6","__REALTIME_TIMESTAMP":"1736164800000000"}`,
			expected: nil,
		},
		{
			name:     "missing message filtered",
			input:    `{"_SYSTEMD_UNIT":"test.service","PRIORITY":"6","__REALTIME_TIMESTAMP":"1736164800000000"}`,
			expected: nil,
		},
		{
			name:     "malformed json skipped",
			input:    `{not valid json}`,
			expected: nil,
		},
		{
			name:     "empty input",
			input:    "",
			expected: nil,
		},
		{
			name: "timestamp fallback",
			input: `{"MESSAGE":"First","PRIORITY":"6","__REALTIME_TIMESTAMP":"1736164800000000"}
{"MESSAGE":"Second","PRIORITY":"6","__REALTIME_TIMESTAMP":""}`,
			expected: []protocol.LogEntry{
				{Timestamp: 1736164800, Source: "journald:unknown", Level: protocol.LevelInfo, Message: "First"},
				{Timestamp: 1736164800, Source: "journald:unknown", Level: protocol.LevelInfo, Message: "Second"},
			},
		},
		{
			name: "all priority levels",
			input: `{"MESSAGE":"emerg","PRIORITY":"0","__REALTIME_TIMESTAMP":"1736164800000000"}
{"MESSAGE":"alert","PRIORITY":"1","__REALTIME_TIMESTAMP":"1736164800000000"}
{"MESSAGE":"crit","PRIORITY":"2","__REALTIME_TIMESTAMP":"1736164800000000"}
{"MESSAGE":"err","PRIORITY":"3","__REALTIME_TIMESTAMP":"1736164800000000"}
{"MESSAGE":"warn","PRIORITY":"4","__REALTIME_TIMESTAMP":"1736164800000000"}
{"MESSAGE":"notice","PRIORITY":"5","__REALTIME_TIMESTAMP":"1736164800000000"}
{"MESSAGE":"info","PRIORITY":"6","__REALTIME_TIMESTAMP":"1736164800000000"}
{"MESSAGE":"debug","PRIORITY":"7","__REALTIME_TIMESTAMP":"1736164800000000"}`,
			expected: []protocol.LogEntry{
				{Timestamp: 1736164800, Source: "journald:unknown", Level: protocol.LevelEmergency, Message: "emerg"},
				{Timestamp: 1736164800, Source: "journald:unknown", Level: protocol.LevelAlert, Message: "alert"},
				{Timestamp: 1736164800, Source: "journald:unknown", Level: protocol.LevelCritical, Message: "crit"},
				{Timestamp: 1736164800, Source: "journald:unknown", Level: protocol.LevelError, Message: "err"},
				{Timestamp: 1736164800, Source: "journald:unknown", Level: protocol.LevelWarning, Message: "warn"},
				{Timestamp: 1736164800, Source: "journald:unknown", Level: protocol.LevelNotice, Message: "notice"},
				{Timestamp: 1736164800, Source: "journald:unknown", Level: protocol.LevelInfo, Message: "info"},
				{Timestamp: 1736164800, Source: "journald:unknown", Level: protocol.LevelDebug, Message: "debug"},
			},
		},
		{
			name:  "empty systemd unit falls back to syslog identifier",
			input: `{"MESSAGE":"Test","_SYSTEMD_UNIT":"","SYSLOG_IDENTIFIER":"fallback","PRIORITY":"6","__REALTIME_TIMESTAMP":"1736164800000000"}`,
			expected: []protocol.LogEntry{
				{Timestamp: 1736164800, Source: "journald:fallback", Level: protocol.LevelInfo, Message: "Test"},
			},
		},
		{
			name: "mixed valid and invalid json",
			input: `{"MESSAGE":"Valid","PRIORITY":"6","__REALTIME_TIMESTAMP":"1736164800000000"}
{invalid json line}
{"MESSAGE":"Also valid","PRIORITY":"6","__REALTIME_TIMESTAMP":"1736164801000000"}`,
			expected: []protocol.LogEntry{
				{Timestamp: 1736164800, Source: "journald:unknown", Level: protocol.LevelInfo, Message: "Valid"},
				{Timestamp: 1736164801, Source: "journald:unknown", Level: protocol.LevelInfo, Message: "Also valid"},
			},
		},
		{
			name:  "unknown priority defaults to info",
			input: `{"MESSAGE":"Test","PRIORITY":"99","__REALTIME_TIMESTAMP":"1736164800000000"}`,
			expected: []protocol.LogEntry{
				{Timestamp: 1736164800, Source: "journald:unknown", Level: protocol.LevelInfo, Message: "Test"},
			},
		},
		{
			name:  "message with special characters",
			input: `{"MESSAGE":"Error: \"file not found\" at /path/to/file","PRIORITY":"3","__REALTIME_TIMESTAMP":"1736164800000000"}`,
			expected: []protocol.LogEntry{
				{Timestamp: 1736164800, Source: "journald:unknown", Level: protocol.LevelError, Message: `Error: "file not found" at /path/to/file`},
			},
		},
		{
			name:  "message with newlines encoded",
			input: `{"MESSAGE":"Line1\nLine2\nLine3","PRIORITY":"6","__REALTIME_TIMESTAMP":"1736164800000000"}`,
			expected: []protocol.LogEntry{
				{Timestamp: 1736164800, Source: "journald:unknown", Level: protocol.LevelInfo, Message: "Line1\nLine2\nLine3"},
			},
		},
		{
			name: "empty lines between entries",
			input: `{"MESSAGE":"First","PRIORITY":"6","__REALTIME_TIMESTAMP":"1736164800000000"}

{"MESSAGE":"Second","PRIORITY":"6","__REALTIME_TIMESTAMP":"1736164801000000"}`,
			expected: []protocol.LogEntry{
				{Timestamp: 1736164800, Source: "journald:unknown", Level: protocol.LevelInfo, Message: "First"},
				{Timestamp: 1736164801, Source: "journald:unknown", Level: protocol.LevelInfo, Message: "Second"},
			},
		},
		{
			name:  "pid as string",
			input: `{"MESSAGE":"Test","_PID":"5678","_COMM":"proc","PRIORITY":"6","__REALTIME_TIMESTAMP":"1736164800000000"}`,
			expected: []protocol.LogEntry{
				{Timestamp: 1736164800, Source: "journald:proc", Level: protocol.LevelInfo, Message: "Test", ProcessName: "proc", ProcessID: 5678},
			},
		},
		{
			name:  "invalid pid ignored",
			input: `{"MESSAGE":"Test","_PID":"notanumber","_COMM":"proc","PRIORITY":"6","__REALTIME_TIMESTAMP":"1736164800000000"}`,
			expected: []protocol.LogEntry{
				{Timestamp: 1736164800, Source: "journald:proc", Level: protocol.LevelInfo, Message: "Test", ProcessName: "proc", ProcessID: 0},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseJournalFrom(strings.NewReader(tt.input), 10000)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != len(tt.expected) {
				t.Fatalf("length mismatch: got %d, want %d", len(got), len(tt.expected))
			}

			for i := range got {
				if got[i].Timestamp != tt.expected[i].Timestamp {
					t.Errorf("[%d] timestamp: got %d, want %d", i, got[i].Timestamp, tt.expected[i].Timestamp)
				}
				if got[i].Source != tt.expected[i].Source {
					t.Errorf("[%d] source: got %q, want %q", i, got[i].Source, tt.expected[i].Source)
				}
				if got[i].Level != tt.expected[i].Level {
					t.Errorf("[%d] level: got %v, want %v", i, got[i].Level, tt.expected[i].Level)
				}
				if got[i].Message != tt.expected[i].Message {
					t.Errorf("[%d] message: got %q, want %q", i, got[i].Message, tt.expected[i].Message)
				}
				if tt.expected[i].ProcessName != "" && got[i].ProcessName != tt.expected[i].ProcessName {
					t.Errorf("[%d] process name: got %q, want %q", i, got[i].ProcessName, tt.expected[i].ProcessName)
				}
				if tt.expected[i].ProcessID != 0 && got[i].ProcessID != tt.expected[i].ProcessID {
					t.Errorf("[%d] process id: got %d, want %d", i, got[i].ProcessID, tt.expected[i].ProcessID)
				}
			}
		})
	}
}

func TestMapLogLevelToJournalPriority(t *testing.T) {
	tests := []struct {
		level    protocol.LogLevel
		expected string
	}{
		{protocol.LevelDebug, "7"},
		{protocol.LevelInfo, "6"},
		{protocol.LevelNotice, "5"},
		{protocol.LevelWarning, "4"},
		{protocol.LevelError, "3"},
		{protocol.LevelCritical, "2"},
		{protocol.LevelAlert, "1"},
		{protocol.LevelEmergency, "0"},
		{protocol.LogLevel("unknown"), "3"}, // unknown defaults to error
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := mapLogLevelToJournalPriority(tt.level)
			if got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
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
		if entry.Message == "" {
			t.Errorf("entry %d: empty message", i)
		}
		if entry.Level == "" {
			t.Errorf("entry %d: empty level", i)
		}
	}
}

func TestFetchLogs_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := FetchLogs(ctx, protocol.LogRequest{MinLevel: protocol.LevelError})
	t.Logf("FetchLogs with cancelled context: %v", err)
}

func TestFetchLogs_MaxLogsLimit(t *testing.T) {
	if MaxLogs != 10000 {
		t.Errorf("MaxLogs changed from expected value: got %d, want 10000", MaxLogs)
	}
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

func BenchmarkParseDmesgLevel(b *testing.B) {
	levels := []string{"emerg", "alert", "crit", "err", "warn", "notice", "info", "debug", "unknown"}

	b.ReportAllocs()
	for b.Loop() {
		for _, level := range levels {
			_ = parseDmesgLevel(level)
		}
	}
}

func BenchmarkParseDmesgTimestampAndMsg(b *testing.B) {
	input := "[Mon Jan  6 12:00:00 2025] Some kernel message here with more text"

	b.ReportAllocs()
	for b.Loop() {
		_, _ = parseDmesgTimestampAndMsg(input)
	}
}

func BenchmarkParseDmesgTimestampAndMsg_NoTimestamp(b *testing.B) {
	input := "Message without any timestamp brackets"

	b.ReportAllocs()
	for b.Loop() {
		_, _ = parseDmesgTimestampAndMsg(input)
	}
}

func BenchmarkBuildDmesgLevelFlag(b *testing.B) {
	levels := []protocol.LogLevel{
		protocol.LevelDebug,
		protocol.LevelInfo,
		protocol.LevelWarning,
		protocol.LevelError,
		protocol.LevelCritical,
	}

	b.ReportAllocs()
	for b.Loop() {
		for _, level := range levels {
			_ = buildDmesgLevelFlag(level)
		}
	}
}

func TestParseDmesgFrom_Limit(t *testing.T) {
	input := `kern  :info  : [Mon Jan  6 12:00:00 2025] Message 1
kern  :info  : [Mon Jan  6 12:00:01 2025] Message 2
kern  :info  : [Mon Jan  6 12:00:02 2025] Message 3
kern  :info  : [Mon Jan  6 12:00:03 2025] Message 4
kern  :info  : [Mon Jan  6 12:00:04 2025] Message 5`

	tests := []struct {
		name  string
		limit int
		want  int
	}{
		{"limit 0", 0, 0},
		{"limit 1", 1, 1},
		{"limit 3", 3, 3},
		{"limit exceeds entries", 100, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDmesgFrom(strings.NewReader(input), tt.limit)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != tt.want {
				t.Errorf("got %d entries, want %d", len(got), tt.want)
			}
		})
	}
}

func BenchmarkParseDmesgFrom_Small(b *testing.B) {
	input := `kern  :info  : [Mon Jan  6 12:00:00 2025] Message one
kern  :warn  : [Mon Jan  6 12:00:01 2025] Message two
kern  :err   : [Mon Jan  6 12:00:02 2025] Message three`

	b.ReportAllocs()
	for b.Loop() {
		_, _ = parseDmesgFrom(strings.NewReader(input), 10000)
	}
}

func BenchmarkParseDmesgFrom_Large(b *testing.B) {
	var sb strings.Builder
	for i := range 1000 {
		sb.WriteString("kern  :info  : [Mon Jan  6 12:00:00 2025] Kernel message number ")
		sb.WriteString(string(rune('0' + i%10)))
		sb.WriteString(" with some additional text\n")
	}
	input := sb.String()

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = parseDmesgFrom(strings.NewReader(input), 10000)
	}
}

func TestParseJournalFrom_Limit(t *testing.T) {
	input := `{"MESSAGE":"Msg 1","PRIORITY":"6","__REALTIME_TIMESTAMP":"1736164800000000"}
{"MESSAGE":"Msg 2","PRIORITY":"6","__REALTIME_TIMESTAMP":"1736164801000000"}
{"MESSAGE":"Msg 3","PRIORITY":"6","__REALTIME_TIMESTAMP":"1736164802000000"}
{"MESSAGE":"Msg 4","PRIORITY":"6","__REALTIME_TIMESTAMP":"1736164803000000"}
{"MESSAGE":"Msg 5","PRIORITY":"6","__REALTIME_TIMESTAMP":"1736164804000000"}`

	tests := []struct {
		name  string
		limit int
		want  int
	}{
		{"limit 0", 0, 0},
		{"limit 1", 1, 1},
		{"limit 3", 3, 3},
		{"limit exceeds entries", 100, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseJournalFrom(strings.NewReader(input), tt.limit)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != tt.want {
				t.Errorf("got %d entries, want %d", len(got), tt.want)
			}
		})
	}
}

func BenchmarkParseJournalFrom_Small(b *testing.B) {
	input := `{"MESSAGE":"First message","_SYSTEMD_UNIT":"test.service","PRIORITY":"6","__REALTIME_TIMESTAMP":"1736164800000000","_COMM":"test","_PID":"1234"}
{"MESSAGE":"Second message","_SYSTEMD_UNIT":"other.service","PRIORITY":"3","__REALTIME_TIMESTAMP":"1736164801000000","_COMM":"other","_PID":"5678"}`

	b.ReportAllocs()
	for b.Loop() {
		_, _ = parseJournalFrom(strings.NewReader(input), 10000)
	}
}

func BenchmarkParseJournalFrom_Large(b *testing.B) {
	var sb strings.Builder
	for i := range 1000 {
		sb.WriteString(`{"MESSAGE":"Log message number `)
		sb.WriteString(string(rune('0' + i%10)))
		sb.WriteString(`","_SYSTEMD_UNIT":"test.service","PRIORITY":"6","__REALTIME_TIMESTAMP":"1736164800000000","_COMM":"test","_PID":"1234"}`)
		sb.WriteString("\n")
	}
	input := sb.String()

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = parseJournalFrom(strings.NewReader(input), 10000)
	}
}

func BenchmarkMapLogLevelToJournalPriority(b *testing.B) {
	levels := []protocol.LogLevel{
		protocol.LevelDebug,
		protocol.LevelInfo,
		protocol.LevelNotice,
		protocol.LevelWarning,
		protocol.LevelError,
		protocol.LevelCritical,
		protocol.LevelAlert,
		protocol.LevelEmergency,
	}

	b.ReportAllocs()
	for b.Loop() {
		for _, level := range levels {
			_ = mapLogLevelToJournalPriority(level)
		}
	}
}

func BenchmarkFetchLogs_Integration(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping integration benchmark")
	}

	ctx := context.Background()

	// Warm up and report count
	logs, err := FetchLogs(ctx, protocol.LogRequest{MinLevel: protocol.LevelError})
	if err != nil {
		b.Skipf("FetchLogs not available: %v", err)
	}
	b.Logf("Benchmarking with ~%d log entries", len(logs))

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = FetchLogs(ctx, protocol.LogRequest{MinLevel: protocol.LevelError})
	}
}

func BenchmarkFetchLogs_Error(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping integration benchmark")
	}
	ctx := context.Background()
	logs, _ := FetchLogs(ctx, protocol.LogRequest{MinLevel: protocol.LevelError})
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
	logs, _ := FetchLogs(ctx, protocol.LogRequest{MinLevel: protocol.LevelWarning})
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
	logs, _ := FetchLogs(ctx, protocol.LogRequest{MinLevel: protocol.LevelInfo})
	b.Logf("Info level: %d entries", len(logs))
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = FetchLogs(ctx, protocol.LogRequest{MinLevel: protocol.LevelInfo})
	}
}

func BenchmarkFetchLogs_AllLevels(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping integration benchmark")
	}

	ctx := context.Background()

	b.ReportAllocs()
	for b.Loop() {
		_, _ = FetchLogs(ctx, protocol.LogRequest{MinLevel: protocol.LevelDebug})
	}
}
