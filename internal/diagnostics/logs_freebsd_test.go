//go:build freebsd && amd64

package diagnostics

import (
	"cmp"
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

const syslogSample = `Feb 13 14:00:00 hp-elite-mini-bsd newsyslog[44528]: logfile turned over due to size>1000K
Feb 13 14:00:00 hp-elite-mini-bsd kernel: linux: jid 0 pid 4247 (ThreadPoolForeg): unsupported TCP socket option TCP_INFO (11)
Feb 13 14:00:00 hp-elite-mini-bsd syslogd: last message repeated 5 times
Feb 13 14:00:37 hp-elite-mini-bsd kernel: linux: jid 0 pid 4251 (DedicatedWorker): unsupported prctl option 1398164801
Feb 19 12:24:47 hp-elite-mini-bsd sshd[1234]: Accepted publickey for nhdewitt from 192.168.1.10 port 52345 ssh2
Feb 19 12:25:17 hp-elite-mini-bsd sshd[5678]: Failed password for invalid user admin from 10.0.0.1 port 22
Feb 19 12:26:00 hp-elite-mini-bsd kernel: error: something went wrong
Feb 19 12:26:30 hp-elite-mini-bsd kernel: WARNING: disk is getting full
`

func sampleLines() []string {
	var lines []string
	for _, l := range strings.Split(strings.TrimRight(syslogSample, "\n"), "\n") {
		if l != "" {
			lines = append(lines, l)
		}
	}
	return lines
}

// --- Timestamp parsing ---

func TestParseSyslogTimestamp(t *testing.T) {
	now := time.Date(2025, time.February, 19, 15, 0, 0, 0, time.UTC)

	tests := []struct {
		name  string
		input string
		want  time.Month
		day   int
	}{
		{"normal", "Feb 19 12:24:47", time.February, 19},
		{"single digit day", "Feb  3 08:00:00", time.February, 3},
		{"different month", "Jan 15 23:59:59", time.January, 15},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ts := parseSyslogTimestamp(tc.input, now)
			if ts == 0 {
				t.Fatal("timestamp parsed as 0")
			}
			parsed := time.Unix(ts, 0).UTC()
			if parsed.Month() != tc.want {
				t.Errorf("month = %v, want %v", parsed.Month(), tc.want)
			}
			if parsed.Day() != tc.day {
				t.Errorf("day = %d, want %d", parsed.Day(), tc.day)
			}
		})
	}
}

func TestParseSyslogTimestampYearBoundary(t *testing.T) {
	now := time.Date(2025, time.January, 5, 12, 0, 0, 0, time.UTC)
	ts := parseSyslogTimestamp("Dec 30 23:59:59", now)
	parsed := time.Unix(ts, 0).UTC()

	if parsed.Year() != 2024 {
		t.Errorf("year = %d, want 2024 (year boundary)", parsed.Year())
	}
}

func TestParseSyslogTimestampInvalid(t *testing.T) {
	now := time.Now()
	ts := parseSyslogTimestamp("not a timestamp", now)
	if ts != 0 {
		t.Errorf("expected 0 for invalid timestamp, got %d", ts)
	}
}

// --- Line parsing ---

func TestParseSyslogLines(t *testing.T) {
	entries, err := parseSyslogLines(sampleLines(), protocol.LevelNotice)
	if err != nil {
		t.Fatal(err)
	}

	for _, e := range entries {
		if strings.Contains(e.Message, "last message repeated") {
			t.Error("should have skipped 'last message repeated' line")
		}
	}

	// 8 lines minus 1 repeated = 7
	if len(entries) != 7 {
		t.Errorf("got %d entries, want 7", len(entries))
	}
}

func TestParseSyslogLinesSource(t *testing.T) {
	entries, err := parseSyslogLines(sampleLines(), protocol.LevelNotice)
	if err != nil {
		t.Fatal(err)
	}

	if entries[0].Source != "syslog:newsyslog" {
		t.Errorf("source = %q, want %q", entries[0].Source, "syslog:newsyslog")
	}
	if entries[0].ProcessID != 44528 {
		t.Errorf("pid = %d, want 44528", entries[0].ProcessID)
	}
	if entries[0].ProcessName != "newsyslog" {
		t.Errorf("process name = %q, want %q", entries[0].ProcessName, "newsyslog")
	}

	if entries[1].Source != "syslog:kernel" {
		t.Errorf("source = %q, want %q", entries[1].Source, "syslog:kernel")
	}
	if entries[1].ProcessID != 0 {
		t.Errorf("pid = %d, want 0", entries[1].ProcessID)
	}
}

func TestParseSyslogLinesSSHDWithPID(t *testing.T) {
	entries, err := parseSyslogLines(sampleLines(), protocol.LevelNotice)
	if err != nil {
		t.Fatal(err)
	}

	var sshEntry *protocol.LogEntry
	for i := range entries {
		if entries[i].ProcessID == 1234 {
			sshEntry = &entries[i]
			break
		}
	}

	if sshEntry == nil {
		t.Fatal("sshd entry with PID 1234 not found")
	}
	if sshEntry.Source != "syslog:sshd" {
		t.Errorf("source = %q, want %q", sshEntry.Source, "syslog:sshd")
	}
	if !strings.Contains(sshEntry.Message, "Accepted publickey") {
		t.Errorf("message = %q, want to contain 'Accepted publickey'", sshEntry.Message)
	}
}

func TestParseSyslogLinesEmpty(t *testing.T) {
	entries, err := parseSyslogLines(nil, protocol.LevelNotice)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("got %d entries for nil input, want 0", len(entries))
	}
}

func TestParseSyslogLinesMalformed(t *testing.T) {
	lines := []string{
		"this is not a syslog line",
		"also not valid",
		"Feb 19 12:00:00 host kernel: valid line",
		"more garbage",
	}
	entries, err := parseSyslogLines(lines, protocol.LevelNotice)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("got %d entries, want 1 (only valid line)", len(entries))
	}
}

// --- Level inference ---

func TestInferLevel(t *testing.T) {
	tests := []struct {
		name         string
		source       string
		msg          string
		defaultLevel protocol.LogLevel
		want         protocol.LogLevel
	}{
		{"kernel error", "kernel", "error: something went wrong", protocol.LevelNotice, protocol.LevelError},
		{"kernel warn", "kernel", "WARNING: disk is getting full", protocol.LevelNotice, protocol.LevelWarning},
		{"kernel normal", "kernel", "linux: jid 0 pid 4247: unsupported option", protocol.LevelNotice, protocol.LevelNotice},
		{"sshd failure", "sshd", "Failed password for invalid user admin", protocol.LevelNotice, protocol.LevelWarning},
		{"sshd success", "sshd", "Accepted publickey for nhdewitt", protocol.LevelNotice, protocol.LevelNotice},
		{"generic source", "newsyslog", "logfile turned over", protocol.LevelNotice, protocol.LevelNotice},
		{"login invalid", "login", "invalid user attempt", protocol.LevelInfo, protocol.LevelWarning},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := inferLevel(tc.source, tc.msg, tc.defaultLevel)
			if got != tc.want {
				t.Errorf("inferLevel(%q, %q) = %v, want %v", tc.source, tc.msg, got, tc.want)
			}
		})
	}
}

// --- Regex ---

func TestRegexParsing(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		wantMatch bool
		wantParts []string // [timestamp, source, pid, message]
	}{
		{
			"with pid",
			"Feb 13 14:00:00 myhost sshd[1234]: connection from 10.0.0.1",
			true,
			[]string{"Feb 13 14:00:00", "sshd", "1234", "connection from 10.0.0.1"},
		},
		{
			"without pid",
			"Feb 19 12:24:47 myhost kernel: some message here",
			true,
			[]string{"Feb 19 12:24:47", "kernel", "", "some message here"},
		},
		{
			"syslogd repeated",
			"Feb 13 14:00:00 myhost syslogd: last message repeated 5 times",
			true,
			[]string{"Feb 13 14:00:00", "syslogd", "", "last message repeated 5 times"},
		},
		{
			"garbage",
			"not a syslog line",
			false,
			nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := reSyslog.FindStringSubmatch(tc.line)
			if tc.wantMatch && m == nil {
				t.Fatal("expected match, got nil")
			}
			if !tc.wantMatch && m != nil {
				t.Fatalf("expected no match, got %v", m)
			}
			if !tc.wantMatch {
				return
			}

			if m[1] != tc.wantParts[0] {
				t.Errorf("timestamp = %q, want %q", m[1], tc.wantParts[0])
			}
			if m[2] != tc.wantParts[1] {
				t.Errorf("source = %q, want %q", m[2], tc.wantParts[1])
			}
			if m[3] != tc.wantParts[2] {
				t.Errorf("pid = %q, want %q", m[3], tc.wantParts[2])
			}
			if m[4] != tc.wantParts[3] {
				t.Errorf("message = %q, want %q", m[4], tc.wantParts[3])
			}
		})
	}
}

// --- readLines ---

func TestReadLines(t *testing.T) {
	f := writeTempFile(t, "line 1\nline 2\nline 3\n")

	lines, err := readLines(f)
	if err != nil {
		t.Fatal(err)
	}

	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3", len(lines))
	}
	if lines[0] != "line 1" || lines[2] != "line 3" {
		t.Errorf("lines = %v", lines)
	}
}

func TestReadLinesEmpty(t *testing.T) {
	f := writeTempFile(t, "")

	lines, err := readLines(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 0 {
		t.Errorf("got %d lines for empty file, want 0", len(lines))
	}
}

func TestReadLinesSkipsEmpty(t *testing.T) {
	f := writeTempFile(t, "line 1\n\n\nline 2\n\n")

	lines, err := readLines(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 2 {
		t.Errorf("got %d lines, want 2 (empty lines skipped)", len(lines))
	}
}

// --- Merge sort / chronological ordering ---

func TestMergedEntriesChronological(t *testing.T) {
	messagesLines := []string{
		"Feb 19 12:00:00 host kernel: first from messages",
		"Feb 19 12:02:00 host kernel: third from messages",
		"Feb 19 12:04:00 host kernel: fifth from messages",
	}
	authLines := []string{
		"Feb 19 12:01:00 host sshd[100]: second from auth",
		"Feb 19 12:03:00 host sshd[101]: fourth from auth",
		"Feb 19 12:05:00 host sshd[102]: sixth from auth",
	}

	msgEntries, err := parseSyslogLines(messagesLines, protocol.LevelNotice)
	if err != nil {
		t.Fatal(err)
	}
	authEntries, err := parseSyslogLines(authLines, protocol.LevelInfo)
	if err != nil {
		t.Fatal(err)
	}

	var all []protocol.LogEntry
	all = append(all, msgEntries...)
	all = append(all, authEntries...)

	if len(all) != 6 {
		t.Fatalf("got %d entries, want 6", len(all))
	}

	slices.SortFunc(all, func(a, b protocol.LogEntry) int {
		return cmp.Compare(a.Timestamp, b.Timestamp)
	})

	// Verify chronological order
	for i := 1; i < len(all); i++ {
		if all[i].Timestamp < all[i-1].Timestamp {
			t.Errorf("entries[%d].Timestamp (%d) < entries[%d].Timestamp (%d)",
				i, all[i].Timestamp, i-1, all[i-1].Timestamp)
		}
	}

	// Verify interleaved order
	wantOrder := []string{"first", "second", "third", "fourth", "fifth", "sixth"}
	for i, want := range wantOrder {
		if !strings.Contains(all[i].Message, want) {
			t.Errorf("entries[%d].Message = %q, want to contain %q", i, all[i].Message, want)
		}
	}
}

func TestMergedEntriesTruncation(t *testing.T) {
	var lines []string
	for i := 0; i < 20; i++ {
		lines = append(lines, fmt.Sprintf("Feb 19 12:%02d:00 host kernel: message %d", i, i))
	}

	entries, err := parseSyslogLines(lines, protocol.LevelNotice)
	if err != nil {
		t.Fatal(err)
	}

	limit := 10
	if len(entries) > limit {
		entries = entries[len(entries)-limit:]
	}

	if len(entries) != 10 {
		t.Fatalf("got %d entries, want 10", len(entries))
	}

	if !strings.Contains(entries[0].Message, "message 10") {
		t.Errorf("first entry = %q, want 'message 10'", entries[0].Message)
	}
	if !strings.Contains(entries[9].Message, "message 19") {
		t.Errorf("last entry = %q, want 'message 19'", entries[9].Message)
	}
}

// --- Integration ---
func TestFetchLogsIntegration(t *testing.T) {
	ctx := context.Background()

	entries, err := FetchLogs(ctx, protocol.LogRequest{
		MinLevel: protocol.LevelNotice,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) == 0 {
		t.Fatal("expected at least some log entries")
	}

	t.Logf("collected %d entries at notice level", len(entries))

	// Should not exceed MaxLogs
	if len(entries) > MaxLogs {
		t.Errorf("got %d entries, exceeds MaxLogs (%d)", len(entries), MaxLogs)
	}

	// Verify chronological order
	for i := 1; i < len(entries); i++ {
		if entries[i].Timestamp < entries[i-1].Timestamp {
			t.Errorf("entries not chronological: [%d].Timestamp=%d < [%d].Timestamp=%d",
				i, entries[i].Timestamp, i-1, entries[i-1].Timestamp)
			break
		}
	}

	// Every entry should have a source
	for i, e := range entries {
		if e.Source == "" {
			t.Errorf("entries[%d] has empty source", i)
			break
		}
	}

	// Every entry should have a message
	for i, e := range entries {
		if e.Message == "" {
			t.Errorf("entries[%d] has empty message", i)
			break
		}
	}

	// No "last message repeated" should leak through
	for i, e := range entries {
		if e.Source != "dmesg:kernel" && containsRepeated(e.Message) {
			t.Errorf("entries[%d] contains 'last message repeated': %q", i, e.Message)
			break
		}
	}

	// Log some sample entries for manual inspection
	limit := 5
	if len(entries) < limit {
		limit = len(entries)
	}
	t.Log("--- first entries ---")
	for _, e := range entries[:limit] {
		t.Logf("  [%s] %s: %s", time.Unix(e.Timestamp, 0).Format(time.DateTime), e.Source, truncate(e.Message, 80))
	}
	t.Log("--- last entries ---")
	for _, e := range entries[len(entries)-limit:] {
		t.Logf("  [%s] %s: %s", time.Unix(e.Timestamp, 0).Format(time.DateTime), e.Source, truncate(e.Message, 80))
	}
}

func TestFetchLogsIntegrationDebugLevel(t *testing.T) {
	ctx := context.Background()

	debugEntries, err := FetchLogs(ctx, protocol.LogRequest{
		MinLevel: protocol.LevelDebug,
	})
	if err != nil {
		t.Fatal(err)
	}

	noticeEntries, err := FetchLogs(ctx, protocol.LogRequest{
		MinLevel: protocol.LevelNotice,
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("debug level: %d entries, notice level: %d entries", len(debugEntries), len(noticeEntries))

	// Debug should return at least as many entries as notice
	// (it includes everything notice does, plus debug.log)
	if len(debugEntries) < len(noticeEntries) {
		t.Errorf("debug (%d) returned fewer entries than notice (%d)",
			len(debugEntries), len(noticeEntries))
	}
}

func TestFetchLogsSources(t *testing.T) {
	ctx := context.Background()

	entries, err := FetchLogs(ctx, protocol.LogRequest{
		MinLevel: protocol.LevelDebug,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Tally sources
	sources := make(map[string]int)
	for _, e := range entries {
		// Group by prefix (dmesg:kernel, syslog:kernel, syslog:sshd, etc.)
		sources[e.Source]++
	}

	t.Log("source distribution:")
	for src, count := range sources {
		t.Logf("  %s: %d", src, count)
	}

	// We should have at least dmesg entries (boot messages always exist)
	if sources["dmesg:kernel"] == 0 {
		// dmesg.boot might not be readable without root
		if _, err := os.ReadFile("/var/run/dmesg.boot"); err != nil {
			t.Skip("cannot read /var/run/dmesg.boot, skipping dmesg check")
		}
		t.Error("expected dmesg:kernel entries")
	}
}

func TestFetchLogsTimestamps(t *testing.T) {
	ctx := context.Background()

	entries, err := FetchLogs(ctx, protocol.LogRequest{
		MinLevel: protocol.LevelNotice,
	})
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	oneYearAgo := now.AddDate(-1, 0, 0)
	syslogCount := 0
	badTimestamps := 0

	for _, e := range entries {
		// dmesg entries have no timestamps, skip them
		if e.Source == "dmesg:kernel" {
			continue
		}
		syslogCount++

		if e.Timestamp == 0 {
			badTimestamps++
			continue
		}

		ts := time.Unix(e.Timestamp, 0)
		if ts.Before(oneYearAgo) || ts.After(now.Add(24*time.Hour)) {
			t.Errorf("suspicious timestamp %v for entry: %s", ts, truncate(e.Message, 60))
			badTimestamps++
		}
	}

	if syslogCount > 0 && badTimestamps > syslogCount/10 {
		t.Errorf("%d/%d syslog entries have bad timestamps", badTimestamps, syslogCount)
	}
	t.Logf("%d syslog entries, %d with bad timestamps", syslogCount, badTimestamps)
}

func TestFetchLogsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	entries, err := FetchLogs(ctx, protocol.LogRequest{
		MinLevel: protocol.LevelDebug,
	})

	// Should return early with context error or partial results
	if err != nil && err != context.Canceled {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Logf("cancelled fetch returned %d entries (err=%v)", len(entries), err)
}

func containsRepeated(s string) bool {
	return len(s) > 0 && (s == "last message repeated" ||
		(len(s) > 22 && s[:22] == "last message repeated "))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// --- Helpers ---

func writeTempFile(t *testing.T, content string) *os.File {
	t.Helper()
	f, err := os.CreateTemp("", "syslog-test-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(f.Name()) })

	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	// Seek back to start so readLines can read it
	f.Seek(0, 0)
	return f
}
