//go:build darwin

package diagnostics

import (
	"context"
	"encoding/json"
	"io"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

const MaxLogs = 10000

// maxDuplicates caps how many times the same source+message combo
// can appear. Kernel error spam can produce thousands of identical
// entries per hour.
const maxDuplicates = 3

// macLogEntry matches the JSON schema output by "log show --style json"
type macLogEntry struct {
	EventMessage     string `json:"eventMessage"`
	MessageType      string `json:"messageType"`
	ProcessID        int    `json:"processID"`
	Timestamp        string `json:"timestamp"`
	Subsystem        string `json:"subsystem"`
	ProcessImagePath string `json:"processImagePath"`
}

func FetchLogs(ctx context.Context, opts protocol.LogRequest) ([]protocol.LogEntry, error) {
	var results []protocol.LogEntry

	// Kernel logs (dmegs equivalent)
	dmesgPredicate := `processImagePath == "/kernel"`
	if dmesg, err := getMacLogsFiltered(ctx, opts.MinLevel, MaxLogs, dmesgPredicate); err == nil {
		results = append(results, dmesg...)
	}

	// System logs (journalctl equivalent)
	// filters out telemetry noise
	syslogPredicate := `processImagePath != "/kernel" AND (messageType == error OR messageType == fault)`
	if journal, err := getMacLogsFiltered(ctx, opts.MinLevel, MaxLogs, syslogPredicate); err == nil {
		results = append(results, journal...)
	}

	// sort chronologically (oldest to newest)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Timestamp < results[j].Timestamp
	})

	// keep MaxLength newest
	if len(results) > MaxLogs {
		results = results[len(results)-MaxLogs:]
	}

	return results, nil
}

func getMacLogsFiltered(ctx context.Context, minLevel protocol.LogLevel, limit int, predicate string) ([]protocol.LogEntry, error) {
	args := []string{"show", "--style", "json", "--last", "4h", "--predicate", predicate}

	minSeverity := levelToPriority(minLevel)

	// cap at info to prevent OOM
	if minSeverity >= 6 {
		args = append(args, "--info")
	}

	cmd := exec.CommandContext(ctx, "log", args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	entries, err := parseMacLogsAndTail(stdout, minLevel, limit)

	_ = cmd.Wait() // clean up the process

	return entries, err
}

func parseMacLogsAndTail(r io.Reader, minLevel protocol.LogLevel, limit int) ([]protocol.LogEntry, error) {
	var buf []protocol.LogEntry
	decoder := json.NewDecoder(r)
	seen := make(map[string]int)

	minSeverity := levelToPriority(minLevel)

	// consume the opening '[' of the array
	_, err := decoder.Token()
	if err != nil {
		return nil, nil // no predicate matches
	}

	for decoder.More() {
		var mEntry macLogEntry
		if err := decoder.Decode(&mEntry); err != nil {
			continue
		}
		if mEntry.EventMessage == "" {
			continue
		}

		level := parseMacLogLevel(mEntry.MessageType)

		if levelToPriority(level) > minSeverity {
			continue
		}

		// dedup: cap identical source+message combos
		dedupKey := mEntry.ProcessImagePath + "|" + mEntry.EventMessage
		if seen[dedupKey] > maxDuplicates {
			continue
		}
		seen[dedupKey]++
		if seen[dedupKey] == maxDuplicates {
			mEntry.EventMessage += " (further duplicates suppressed)"
		}

		var unixTs int64
		t, err := time.Parse("2006-01-02 15:04:05.999999-0700", mEntry.Timestamp)
		if err == nil {
			unixTs = t.Unix()
		}

		source := mEntry.Subsystem
		if source == "" {
			source = filepath.Base(mEntry.ProcessImagePath)
		}

		if source == "kernel" {
			source = "dmesg:kernel"
		} else {
			source = "unified:" + source
		}

		buf = append(buf, protocol.LogEntry{
			Timestamp:   unixTs,
			Source:      source,
			Level:       level,
			Message:     mEntry.EventMessage,
			ProcessName: filepath.Base(mEntry.ProcessImagePath),
			ProcessID:   mEntry.ProcessID,
		})
	}

	// return the tail
	if len(buf) > limit {
		return buf[len(buf)-limit:], nil
	}

	return buf, nil
}

func parseMacLogLevel(macType string) protocol.LogLevel {
	switch strings.ToLower(macType) {
	case "fault":
		return protocol.LevelCritical
	case "error":
		return protocol.LevelError
	case "info":
		return protocol.LevelInfo
	case "debug":
		return protocol.LevelDebug
	case "default":
		return protocol.LevelNotice
	default:
		return protocol.LevelInfo
	}
}

func levelToPriority(l protocol.LogLevel) int {
	switch l {
	case protocol.LevelEmergency:
		return 0
	case protocol.LevelAlert:
		return 1
	case protocol.LevelCritical:
		return 2
	case protocol.LevelError:
		return 3
	case protocol.LevelWarning:
		return 4
	case protocol.LevelNotice:
		return 5
	case protocol.LevelInfo:
		return 6
	case protocol.LevelDebug:
		return 7
	default:
		return 6
	}
}
