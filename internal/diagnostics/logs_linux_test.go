//go:build !windows

package diagnostics

import (
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDmesgFrom(strings.NewReader(tt.input))
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
			name:  "priority as float",
			input: `{"MESSAGE":"Test","PRIORITY":3,"__REALTIME_TIMESTAMP":"1736164800000000"}`,
			expected: []protocol.LogEntry{
				{Timestamp: 1736164800, Source: "journald:unknown", Level: protocol.LevelError, Message: "Test"},
			},
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseJournalFrom(strings.NewReader(tt.input))
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
