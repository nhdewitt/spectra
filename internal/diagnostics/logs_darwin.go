//go:build darwin

package diagnostics

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

const MaxLogs = 10000

// macLogTimeLayout is what `log show --start/--end` accepts. It is
// interpreted in the host's local zone, so the bound is converted from
// Unix seconds through time.Unix rather than being formatted as UTC.
const macLogTimeLayout = "2006-01-02 15:04:05"

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

	limit := gatherLimit(opts, MaxLogs)

	// Kernel logs (dmegs equivalent)
	dmesgPredicate := `processImagePath == "/kernel"`
	dmesg, err := getMacLogsFiltered(ctx, opts, limit, dmesgPredicate)
	results = append(results, dmesg...)
	if err != nil {
		slog.Warn("kernel log read incomplete", "error", err, "entries", len(dmesg))
	}

	// System logs (journalctl equivalent)
	// filters out telemetry noise
	syslogPredicate := `processImagePath != "/kernel" AND (messageType == error OR messageType == fault)`
	journal, err := getMacLogsFiltered(ctx, opts, limit, syslogPredicate)
	results = append(results, journal...)
	if err != nil {
		slog.Warn("unified log read incomplete", "error", err, "entries", len(journal))
	}

	return finalize(results, opts, MaxLogs), nil
}

func getMacLogsFiltered(ctx context.Context, opts protocol.LogRequest, limit int, predicate string) ([]protocol.LogEntry, error) {
	args := []string{"show", "--style", "json", "--predicate", predicate}

	// Fall back to the previous fixed window when the request carries no lower bound. An unbounded
	// `log show` on a busy Mac is minutes of output, so the absence of a bound can't mean all of it.
	if opts.Since > 0 {
		args = append(args, "--start", time.Unix(opts.Since, 0).Format(macLogTimeLayout))
	} else {
		args = append(args, "--last", "4h")
	}
	if opts.Until > 0 {
		args = append(args, "--end", time.Unix(opts.Until, 0).Format(macLogTimeLayout))
	}

	// cap at info to prevent OOM
	if levelToPriority(opts.MinLevel) >= 6 {
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

	entries, err := parseMacLogsAndTail(stdout, opts.MinLevel, limit)

	if waitErr := cmd.Wait(); err == nil {
		err = waitErr
	}

	return entries, err
}

func parseMacLogsAndTail(r io.Reader, minLevel protocol.LogLevel, limit int) ([]protocol.LogEntry, error) {
	var buf []protocol.LogEntry
	decoder := json.NewDecoder(r)

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
