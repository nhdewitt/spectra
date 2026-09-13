//go:build freebsd && amd64

package diagnostics

import (
	"bufio"
	"bytes"
	"context"
	"log/slog"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
	"golang.org/x/sys/unix"
)

const MaxLogs = 10000

// logSources maps a log file to the minimum level its entries represent.
// Derived from the default FreeBSD /etc/syslog.
var logSources = []struct {
	path  string
	level protocol.LogLevel
}{
	{"/var/log/messages", protocol.LevelNotice},
	{"/var/log/auth.log", protocol.LevelInfo},
	{"/var/log/debug.log", protocol.LevelDebug},
	{"/var/log/daemon.log", protocol.LevelInfo},
	{"/var/log/security", protocol.LevelInfo},
}

var reSyslog = regexp.MustCompile(
	`^(\w{3}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2})\s+\S+\s+(\S+?)(?:\[(\d+)])?:\s+(.+)$`,
)

func FetchLogs(ctx context.Context, opts protocol.LogRequest) ([]protocol.LogEntry, error) {
	var results []protocol.LogEntry

	minPriority := levelToPriority(opts.MinLevel)

	dmesg, err := getDmesg()
	results = append(results, dmesg...)
	if err != nil && !os.IsNotExist(err) {
		slog.Warn("dmesg.boot read incomplete", "error", err, "entries", len(dmesg))
	}

	for _, src := range logSources {
		entries, err := parseSyslogFile(src.path, src.level)
		if err != nil {
			if !os.IsNotExist(err) {
				slog.Warn("syslog read incomplete", "path", src.path, "error", err, "entries", len(entries))
			}
			if len(entries) == 0 {
				continue
			}
		}
		results = append(results, entries...)
	}

	// Filter on each entry's own level. This is the only severity filter on this platform.
	// The other three push the level down into journalctl, Get-WinEvent, or the log predicate
	// and arrive pre-filtered, which is why finalize stays level-agnostic rather than doing
	// this for everyone.
	kept := results[:0]
	for _, e := range results {
		if levelToPriority(e.Level) <= minPriority {
			kept = append(kept, e)
		}
	}
	results = kept

	return finalize(results, opts, MaxLogs), nil
}

func getDmesg() ([]protocol.LogEntry, error) {
	data, err := os.ReadFile("/var/run/dmesg.boot")
	if err != nil {
		return nil, err
	}

	// Use boottime as the timestamp for all dmesg entries
	var bootTime int64
	if tv, err := unix.SysctlTimeval("kern.boottime"); err == nil {
		bootTime = tv.Sec
	}

	var entries []protocol.LogEntry
	scanner := bufio.NewScanner(bytes.NewReader(data))

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "---") {
			continue
		}

		entries = append(entries, protocol.LogEntry{
			Timestamp: bootTime,
			Source:    "dmesg:kernel",
			Level:     protocol.LevelNotice,
			Message:   line,
		})
	}

	return entries, scanner.Err()
}

func parseSyslogFile(path string, defaultLevel protocol.LogLevel) ([]protocol.LogEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// readLines returns whatever it scanned alongside the error, so a long line late
	// in the file costs that line rather than the whole file.
	lines, err := readLines(f)
	if err != nil && len(lines) == 0 {
		return nil, err
	}

	return parseSyslogLines(lines, defaultLevel)
}

func readLines(f *os.File) ([]string, error) {
	var lines []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		if line := scanner.Text(); line != "" {
			lines = append(lines, line)
		}
	}

	return lines, scanner.Err()
}

func parseSyslogLines(lines []string, defaultLevel protocol.LogLevel) ([]protocol.LogEntry, error) {
	var entries []protocol.LogEntry
	now := time.Now()

	for _, line := range lines {
		if strings.Contains(line, "last message repeated") {
			continue
		}

		m := reSyslog.FindStringSubmatch(line)
		if m == nil {
			continue
		}

		ts := parseSyslogTimestamp(m[1], now)
		source := m[2]
		pid, _ := strconv.Atoi(m[3])
		msg := m[4]

		level := inferLevel(source, msg, defaultLevel)

		entries = append(entries, protocol.LogEntry{
			Timestamp:   ts,
			Source:      "syslog:" + source,
			Level:       level,
			Message:     msg,
			ProcessName: source,
			ProcessID:   pid,
		})
	}

	return entries, nil
}

// parseSyslogTimestamp parses "Feb 19 12:24:47" with the current year.
// BSD syslog timestamps don't include the year.
func parseSyslogTimestamp(s string, now time.Time) int64 {
	t, err := time.Parse("Jan _2 15:04:05", s)
	if err != nil {
		return 0
	}
	// Add current year
	t = t.AddDate(now.Year(), 0, 0)
	if t.After(now.Add(24 * time.Hour)) {
		t = t.AddDate(-1, 0, 0)
	}
	return t.Unix()
}

// inferLevel attempts to determine the log level from the source
// and message content, falling back to defaultLevel.
func inferLevel(source, msg string, defaultLevel protocol.LogLevel) protocol.LogLevel {
	lower := strings.ToLower(msg)

	if source == "kernel" {
		if strings.Contains(lower, "error") || strings.Contains(lower, "fail") {
			return protocol.LevelError
		}
		if strings.Contains(lower, "warn") {
			return protocol.LevelWarning
		}
	}

	// Auth failures
	if source == "sshd" || source == "login" || source == "su" {
		if strings.Contains(lower, "fail") || strings.Contains(lower, "invalid") {
			return protocol.LevelWarning
		}
	}

	return defaultLevel
}
