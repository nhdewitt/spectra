//go:build linux

package diagnostics

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

var dmesgLevels = []string{
	"debug",
	"info",
	"notice",
	"warn",
	"err",
	"crit",
	"alert",
	"emerg",
}

const MaxLogs = 10000

type journalEntry struct {
	Message           string `json:"MESSAGE"`
	SystemdUnit       string `json:"_SYSTEMD_UNIT"`
	SyslogIdentifier  string `json:"SYSLOG_IDENTIFIER"`
	Comm              string `json:"_COMM"`
	PID               string `json:"_PID"`
	Priority          string `json:"PRIORITY"`
	RealtimeTimestamp string `json:"__REALTIME_TIMESTAMP"`
}

func FetchLogs(ctx context.Context, opts protocol.LogRequest) ([]protocol.LogEntry, error) {
	var results []protocol.LogEntry
	remaining := MaxLogs

	// Kernel Logs
	if dmesg, err := getDmesg(ctx, opts.MinLevel, remaining); err == nil {
		results = append(results, dmesg...)
		remaining -= len(dmesg)
	}

	// Journal Logs
	if remaining > 0 {
		if journal, err := getJournal(ctx, opts.MinLevel, remaining); err == nil {
			results = append(results, journal...)
		}
	}

	if len(results) > MaxLogs {
		results = results[len(results)-MaxLogs:]
	}

	return results, nil
}

func getDmesg(ctx context.Context, minLevel protocol.LogLevel, limit int) ([]protocol.LogEntry, error) {
	levelFlag := buildDmesgLevelFlag(minLevel)
	cmd := exec.CommandContext(ctx, "dmesg", "-T", "-x", "--level="+levelFlag)

	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	return parseDmesgFrom(bytes.NewReader(out), limit)
}

func getJournal(ctx context.Context, minLevel protocol.LogLevel, limit int) ([]protocol.LogEntry, error) {
	priority := mapLogLevelToJournalPriority(minLevel)

	cmd := exec.CommandContext(ctx, "journalctl",
		"-b",
		"-p", priority,
		"-n", strconv.Itoa(limit),
		"-o", "json",
		"--no-pager",
	)

	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	return parseJournalFrom(bytes.NewReader(out), limit)
}

// buildDmesgLevelFlag returns a comma-separated string of all levels
// matching or exceeding the requested severity.
func buildDmesgLevelFlag(min protocol.LogLevel) string {
	startIdx := 4

	switch min {
	case protocol.LevelDebug:
		startIdx = 0
	case protocol.LevelInfo:
		startIdx = 1
	case protocol.LevelNotice:
		startIdx = 2
	case protocol.LevelWarning:
		startIdx = 3
	case protocol.LevelError:
		startIdx = 4
	case protocol.LevelCritical:
		startIdx = 5
	case protocol.LevelAlert:
		startIdx = 6
	case protocol.LevelEmergency:
		startIdx = 7
	}

	return strings.Join(dmesgLevels[startIdx:], ",")
}

// parseDmesgFrom parses the raw output of `dmesg -T -x`
func parseDmesgFrom(r io.Reader, limit int) ([]protocol.LogEntry, error) {
	var entries []protocol.LogEntry
	scanner := bufio.NewScanner(r)

	var sourceBuilder strings.Builder
	sourceBuilder.Grow(64)

	// State for sequential timestamp fallback
	var lastTimestamp int64 = 0

	for scanner.Scan() {
		if len(entries) >= limit {
			break
		}
		line := scanner.Text()

		parts := strings.SplitN(line, ":", 3)
		if len(parts) != 3 {
			continue
		}

		level := parseDmesgLevel(strings.TrimSpace(parts[1]))
		raw := strings.TrimSpace(parts[2])
		timestamp, msg := parseDmesgTimestampAndMsg(raw)

		if timestamp == 0 {
			timestamp = lastTimestamp
		} else {
			lastTimestamp = timestamp
		}

		if msg == "" {
			continue
		}

		sourceBuilder.Reset()
		sourceBuilder.WriteString("dmesg:")
		facility := strings.TrimSpace(parts[0])
		if facility == "kern" {
			sourceBuilder.WriteString("kernel")
		} else {
			sourceBuilder.WriteString(facility)
		}

		entries = append(entries, protocol.LogEntry{
			Timestamp: timestamp,
			Source:    sourceBuilder.String(),
			Level:     level,
			Message:   msg,
		})
	}

	return entries, nil
}

func parseDmesgLevel(level string) protocol.LogLevel {
	switch level {
	case "emerg":
		return protocol.LevelEmergency
	case "alert":
		return protocol.LevelAlert
	case "crit":
		return protocol.LevelCritical
	case "err":
		return protocol.LevelError
	case "warn":
		return protocol.LevelWarning
	case "notice":
		return protocol.LevelNotice
	case "info":
		return protocol.LevelInfo
	case "debug":
		return protocol.LevelDebug
	default:
		return protocol.LevelInfo
	}
}

// parseDmesgTimestampAndMsg extracts "[<DATE>] <MESSAGE>"
func parseDmesgTimestampAndMsg(raw string) (int64, string) {
	start := strings.Index(raw, "[")
	end := strings.Index(raw, "]")

	timestamp := int64(0)
	msg := raw

	if start != -1 && end != -1 && end > start {
		timeStr := raw[start+1 : end]

		if parsed, err := time.Parse(time.ANSIC, strings.TrimSpace(timeStr)); err == nil {
			timestamp = parsed.Unix()
		}

		if len(raw) > end+1 {
			msg = strings.TrimSpace(raw[end+1:])
		} else {
			msg = ""
		}
	}

	return timestamp, msg
}

// parseJournalFrom reads JSON from journalctl -o json
func parseJournalFrom(r io.Reader, limit int) ([]protocol.LogEntry, error) {
	var entries []protocol.LogEntry
	scanner := bufio.NewScanner(r)
	var sourceBuilder strings.Builder
	var lastTimestamp int64 = 0

	sourceBuilder.Grow(64)

	for scanner.Scan() {
		if len(entries) >= limit {
			break
		}
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var jEntry journalEntry
		if err := json.Unmarshal(line, &jEntry); err != nil {
			continue
		}

		if jEntry.Message == "" {
			continue
		}

		sourceBuilder.Reset()
		sourceBuilder.WriteString("journald:")

		if jEntry.SystemdUnit != "" {
			sourceBuilder.WriteString(jEntry.SystemdUnit)
		} else if jEntry.SyslogIdentifier != "" {
			sourceBuilder.WriteString(jEntry.SyslogIdentifier)
		} else if jEntry.Comm != "" {
			sourceBuilder.WriteString(jEntry.Comm)
		} else {
			sourceBuilder.WriteString("unknown")
		}

		pid, _ := strconv.Atoi(jEntry.PID)

		level := protocol.LevelInfo
		if jEntry.Priority != "" {
			if priorityInt, err := strconv.Atoi(jEntry.Priority); err == nil {
				if l, exists := protocol.PriorityToLevel[priorityInt]; exists {
					level = l
				}
			}
		}

		var timestamp int64
		if timestampInt, err := strconv.ParseInt(jEntry.RealtimeTimestamp, 10, 64); err == nil {
			timestamp = timestampInt / 1000000
		}

		if timestamp == 0 {
			timestamp = lastTimestamp
		} else {
			lastTimestamp = timestamp
		}

		entries = append(entries, protocol.LogEntry{
			Timestamp:   timestamp,
			Source:      sourceBuilder.String(),
			Level:       level,
			Message:     jEntry.Message,
			ProcessName: jEntry.Comm,
			ProcessID:   pid,
		})
	}

	return entries, nil
}

func mapLogLevelToJournalPriority(l protocol.LogLevel) string {
	switch l {
	case protocol.LevelDebug:
		return "7"
	case protocol.LevelInfo:
		return "6"
	case protocol.LevelNotice:
		return "5"
	case protocol.LevelWarning:
		return "4"
	case protocol.LevelError:
		return "3"
	case protocol.LevelCritical:
		return "2"
	case protocol.LevelAlert:
		return "1"
	case protocol.LevelEmergency:
		return "0"
	default:
		return "3"
	}
}
