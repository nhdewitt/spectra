//go:build !windows

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

func FetchLogs(ctx context.Context, opts protocol.LogRequest) ([]protocol.LogEntry, error) {
	var results []protocol.LogEntry

	// Kernel Logs
	if dmesg, err := getDmesg(ctx, opts.Lines/2, opts.MinLevel, opts.Since); err == nil {
		results = append(results, dmesg...)
	}

	// Journal Logs
	if journal, err := getJournal(ctx, opts.Lines/2, opts.MinLevel, opts.Since); err == nil {
		results = append(results, journal...)
	}

	return results, nil
}

func getDmesg(ctx context.Context, count int, minLevel protocol.LogLevel, since int64) ([]protocol.LogEntry, error) {
	levelFlag := buildDmesgLevelFlag(minLevel)

	cmd := exec.CommandContext(ctx, "dmesg", "-T", "-x", "--level="+levelFlag)

	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	return parseDmesgFrom(bytes.NewReader(out), count, since)
}

func getJournal(ctx context.Context, count int, minLevel protocol.LogLevel, since int64) ([]protocol.LogEntry, error) {
	priority := mapLogLevelToJournalPriority(minLevel)

	args := []string{
		"-n", strconv.Itoa(count),
		"-p", priority,
		"-o", "json",
		"--no-pager",
	}

	if since > 0 {
		startTime := time.Unix(since, 0).Format("2006-01-02 15:04:05")
		args = append(args, "--since", startTime)
	}

	cmd := exec.CommandContext(ctx, "journalctl", args...)

	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	return parseJournalFrom(bytes.NewReader(out))
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
func parseDmesgFrom(r io.Reader, count int, since int64) ([]protocol.LogEntry, error) {
	var entries []protocol.LogEntry
	scanner := bufio.NewScanner(r)
	var allLines []string

	for scanner.Scan() {
		allLines = append(allLines, scanner.Text())
	}

	start := 0
	if len(allLines) > count {
		start = len(allLines) - count
	}

	var sourceBuilder strings.Builder
	sourceBuilder.Grow(32)

	// State for sequential timestamp fallback
	var lastTimestamp int64 = 0

	for i := start; i < len(allLines); i++ {
		line := allLines[i]

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

		if timestamp < since || msg == "" {
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
func parseJournalFrom(r io.Reader) ([]protocol.LogEntry, error) {
	var entries []protocol.LogEntry
	scanner := bufio.NewScanner(r)
	var sourceBuilder strings.Builder
	var lastTimestamp int64 = 0

	sourceBuilder.Grow(64)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var jEntry map[string]interface{}
		if err := json.Unmarshal(line, &jEntry); err != nil {
			continue
		}

		msg, _ := jEntry["MESSAGE"].(string)
		if msg == "" {
			continue
		}

		sourceBuilder.Reset()
		sourceBuilder.WriteString("journald:")

		if unit, ok := jEntry["_SYSTEMD_UNIT"].(string); ok && unit != "" {
			sourceBuilder.WriteString(unit)
		} else if ident, ok := jEntry["SYSLOG_IDENTIFIER"].(string); ok && ident != "" {
			sourceBuilder.WriteString(ident)
		} else if comm, ok := jEntry["_COMM"].(string); ok && comm != "" {
			sourceBuilder.WriteString(comm)
		} else {
			sourceBuilder.WriteString("unknown")
		}

		cmd, _ := jEntry["_COMM"].(string)
		pidStr, _ := jEntry["_PID"].(string)
		pid, _ := strconv.Atoi(pidStr)

		level := protocol.LevelInfo
		if rawPriority, ok := jEntry["PRIORITY"]; ok {
			priorityInt := -1
			switch v := rawPriority.(type) {
			case string:
				priorityInt, _ = strconv.Atoi(v)
			case float64:
				priorityInt = int(v)
			}
			if l, exists := protocol.PriorityToLevel[priorityInt]; exists {
				level = l
			}
		}

		timestampRaw, _ := jEntry["__REALTIME_TIMESTAMP"].(string)
		var timestamp int64
		if timestampInt, err := strconv.ParseInt(timestampRaw, 10, 64); err == nil {
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
			Message:     msg,
			ProcessName: cmd,
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
