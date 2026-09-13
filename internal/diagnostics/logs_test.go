package diagnostics

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/nhdewitt/spectra/internal/protocol"
)

// entriesOfSize builds n entries whose marshalled form is roughly `size` bytes
// each, so a test can aim at the budget without depending on real log content.
// Messages are distinct, so collapseDuplicates is a no-op on them.
func entriesOfSize(n, size int) []protocol.LogEntry {
	out := make([]protocol.LogEntry, n)
	for i := range out {
		out[i] = protocol.LogEntry{
			Timestamp: int64(i),
			Message:   fmt.Sprintf("%06d-%s", i, strings.Repeat("x", size)),
		}
	}
	return out
}

// entry is shorthand for the collapse and window tests, where the timestamp and
// the source/message pair are the only fields that matter.
func entry(ts int64, source, message string) protocol.LogEntry {
	return protocol.LogEntry{
		Timestamp: ts,
		Source:    source,
		Level:     protocol.LevelError,
		Message:   message,
	}
}

func marshalLen(t *testing.T, entries []protocol.LogEntry) int {
	t.Helper()
	b, err := json.Marshal(entries)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return len(b)
}

func timestamps(entries []protocol.LogEntry) []int64 {
	out := make([]int64, len(entries))
	for i, e := range entries {
		out[i] = e.Timestamp
	}
	return out
}

func TestTrimToBudget_FitsUnderTheBudget(t *testing.T) {
	// Comfortably over: ~8MB against a 4MB ceiling, the shape that was being
	// rejected with 413 on every attempt.
	entries := entriesOfSize(20000, 400)
	if got := marshalLen(t, entries); got <= logBudgetBytes {
		t.Fatalf("fixture is not over budget: %d <= %d", got, logBudgetBytes)
	}

	trimmed := trimToBudget(entries)

	if got := marshalLen(t, trimmed); got > logBudgetBytes {
		t.Errorf("still over budget after trim: %d > %d", got, logBudgetBytes)
	}
	if len(trimmed) == 0 {
		t.Error("trimmed to nothing")
	}
}

func TestTrimToBudget_KeepsNewest(t *testing.T) {
	entries := entriesOfSize(20000, 400)
	want := entries[len(entries)-1].Timestamp

	trimmed := trimToBudget(entries)

	// A log view is read from the recent end, so an over-long result must lose
	// its oldest entries rather than its newest.
	if got := trimmed[len(trimmed)-1].Timestamp; got != want {
		t.Errorf("newest entry lost: last timestamp %d, want %d", got, want)
	}
	if trimmed[0].Timestamp <= entries[0].Timestamp && len(trimmed) < len(entries) {
		t.Errorf("trimmed from the wrong end: first timestamp %d", trimmed[0].Timestamp)
	}
}

func TestTrimToBudget_LeavesSmallResultsAlone(t *testing.T) {
	entries := entriesOfSize(50, 200)
	before := marshalLen(t, entries)

	trimmed := trimToBudget(entries)

	if len(trimmed) != len(entries) {
		t.Errorf("entries: got %d, want %d — a result under budget must not be touched", len(trimmed), len(entries))
	}
	if after := marshalLen(t, trimmed); after != before {
		t.Errorf("size changed: %d -> %d", before, after)
	}
}

func TestTrimToBudget_EmptyAndSingle(t *testing.T) {
	if got := trimToBudget(nil); len(got) != 0 {
		t.Errorf("nil input: got %d entries, want 0", len(got))
	}

	// A single entry larger than the whole budget still comes back: dropping it
	// would report an empty log rather than an over-large one. It is truncated
	// instead, so the result both survives and fits.
	huge := entriesOfSize(1, logBudgetBytes*2)
	got := trimToBudget(huge)
	if len(got) != 1 {
		t.Fatalf("single oversized entry: got %d entries, want 1", len(got))
	}
	if size := marshalLen(t, got); size > logBudgetBytes {
		t.Errorf("single entry still over budget: %d > %d", size, logBudgetBytes)
	}
	if !strings.HasSuffix(got[0].Message, truncatedSuffix) {
		t.Error("truncated message should carry the truncation marker")
	}
}

func TestTrimToBudget_ConvergesQuickly(t *testing.T) {
	// Sizing the cut from the overshoot should settle in a couple of passes
	// rather than walking down one entry at a time; at 20k entries the
	// difference is the test finishing or not.
	entries := entriesOfSize(20000, 400)
	trimmed := trimToBudget(entries)

	if len(trimmed) < len(entries)/4 {
		t.Errorf("over-trimmed: kept %d of %d, expected the cut to land near the budget",
			len(trimmed), len(entries))
	}
}

// TestTruncateOversized_RuneBoundary guards the cut position. A message sliced
// mid-rune marshals as U+FFFD, which is three bytes where the fragment may have
// been one, so a naive cut can grow the payload it was shortening.
func TestTruncateOversized_RuneBoundary(t *testing.T) {
	// Multi-byte runes throughout, sized past the budget on its own.
	msg := strings.Repeat("\u65e5", logBudgetBytes)
	e := protocol.LogEntry{Timestamp: 1, Source: "s", Message: msg}

	got := truncateOversized(e)

	if !utf8.ValidString(got.Message) {
		t.Error("truncation produced invalid UTF-8")
	}
	if strings.ContainsRune(got.Message, utf8.RuneError) {
		t.Error("truncation cut mid-rune and produced U+FFFD")
	}
	if size := marshalLen(t, []protocol.LogEntry{got}); size > logBudgetBytes {
		t.Errorf("still over budget: %d > %d", size, logBudgetBytes)
	}
}

func TestLogBudgetIsUnderTheServerLimit(t *testing.T) {
	// The margin exists because the server measures the command-result envelope
	// around these entries, not the entries alone.
	if logBudgetBytes >= protocol.MaxCommandResultBytes {
		t.Errorf("logBudgetBytes (%d) must stay under protocol.MaxCommandResultBytes (%d)",
			logBudgetBytes, protocol.MaxCommandResultBytes)
	}
}

func TestDedupKey_IgnoresTimestamp(t *testing.T) {
	// Folding occurrences that are minutes or days apart is the entire point of
	// collapsing, so the key must not include time.
	a := dedupKey("WinEvent:MsiInstaller", "Error 1704. An installation is suspended.")
	b := dedupKey("WinEvent:MsiInstaller", "Error 1704. An installation is suspended.")
	if a != b {
		t.Errorf("identical messages produced different keys: %q vs %q", a, b)
	}

	if dedupKey("WinEvent:MsiInstaller", "x") == dedupKey("WinEvent:Other", "x") {
		t.Error("different sources collided")
	}
	if dedupKey("s", "a") == dedupKey("s", "b") {
		t.Error("different messages collided")
	}
}

// TestDedupKey_NoSeparatorCollision guards the concatenation: without a
// separator, ("ab","c") and ("a","bc") would be the same key.
func TestDedupKey_NoSeparatorCollision(t *testing.T) {
	if dedupKey("ab", "c") == dedupKey("a", "bc") {
		t.Error("source and message boundary is ambiguous")
	}
}

// TestCollapseDuplicates_AnchorsOnMostRecent pins the choice that makes
// collapsing survivable: the kept entry is the LAST occurrence, not the first.
// Both the byte trim and every platform cap discard from the old end, so an
// entry anchored at its first occurrence sits exactly where the cuts land.
func TestCollapseDuplicates_AnchorsOnMostRecent(t *testing.T) {
	var entries []protocol.LogEntry
	for ts := int64(1); ts <= 5; ts++ {
		entries = append(entries, entry(ts, "WinEvent:MsiInstaller", "Error 1704"))
	}

	got := collapseDuplicates(entries)

	if len(got) != 1 {
		t.Fatalf("collapsed to %d entries, want 1", len(got))
	}
	if got[0].Timestamp != 5 {
		t.Errorf("kept occurrence at %d, want the most recent (5)", got[0].Timestamp)
	}
	if got[0].FirstSeen != 1 {
		t.Errorf("FirstSeen = %d, want the oldest occurrence (1)", got[0].FirstSeen)
	}
	if got[0].Count != 5 {
		t.Errorf("Count = %d, want 5", got[0].Count)
	}
}

// TestCollapseDuplicates_SinglesUnannotated keeps a one-off entry byte-identical
// to what agents sent before collapsing existed; both fields are omitempty.
func TestCollapseDuplicates_SinglesUnannotated(t *testing.T) {
	entries := []protocol.LogEntry{
		entry(1, "journald:sshd", "alpha"),
		entry(2, "journald:sshd", "beta"),
	}

	got := collapseDuplicates(entries)

	if len(got) != 2 {
		t.Fatalf("collapsed distinct messages: got %d entries, want 2", len(got))
	}
	for _, e := range got {
		if e.Count != 0 || e.FirstSeen != 0 {
			t.Errorf("%q: Count=%d FirstSeen=%d, want both zero on a single occurrence",
				e.Message, e.Count, e.FirstSeen)
		}
	}

	b, err := json.Marshal(got[0])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(b), "count") || strings.Contains(string(b), "first_seen") {
		t.Errorf("single entry serialized the collapse fields: %s", b)
	}
}

// TestCollapseDuplicates_PreservesOrder checks that interleaved runs come back
// ascending by the timestamp of each kept occurrence, since the walk runs
// backward and reverses at the end.
func TestCollapseDuplicates_PreservesOrder(t *testing.T) {
	entries := []protocol.LogEntry{
		entry(1, "s", "a"),
		entry(2, "s", "b"),
		entry(3, "s", "a"),
		entry(4, "s", "b"),
		entry(5, "s", "c"),
	}

	got := collapseDuplicates(entries)

	want := []int64{3, 4, 5}
	gotTS := timestamps(got)
	if len(gotTS) != len(want) {
		t.Fatalf("got %d entries %v, want %d %v", len(gotTS), gotTS, len(want), want)
	}
	for i := range want {
		if gotTS[i] != want[i] {
			t.Fatalf("timestamps = %v, want %v", gotTS, want)
		}
	}

	for _, e := range got {
		switch e.Message {
		case "a":
			if e.Count != 2 || e.FirstSeen != 1 {
				t.Errorf("a: Count=%d FirstSeen=%d, want 2 and 1", e.Count, e.FirstSeen)
			}
		case "b":
			if e.Count != 2 || e.FirstSeen != 2 {
				t.Errorf("b: Count=%d FirstSeen=%d, want 2 and 2", e.Count, e.FirstSeen)
			}
		case "c":
			if e.Count != 0 {
				t.Errorf("c: Count=%d, want 0", e.Count)
			}
		}
	}
}

func TestCollapseDuplicates_EmptyAndSingle(t *testing.T) {
	if got := collapseDuplicates(nil); len(got) != 0 {
		t.Errorf("nil input: got %d entries, want 0", len(got))
	}

	one := []protocol.LogEntry{entry(1, "s", "a")}
	got := collapseDuplicates(one)
	if len(got) != 1 {
		t.Fatalf("single entry: got %d, want 1", len(got))
	}
	if got[0].Count != 0 || got[0].FirstSeen != 0 {
		t.Errorf("single entry annotated: Count=%d FirstSeen=%d", got[0].Count, got[0].FirstSeen)
	}
}

// TestFinalize_CollapsesBeforeLimiting is the ordering that makes the rework
// worth doing. Collapsing discards nothing, so it must run before the limit;
// limiting first would throw away distinct entries to make room for copies of
// one message.
func TestFinalize_CollapsesBeforeLimiting(t *testing.T) {
	var entries []protocol.LogEntry
	for ts := int64(1); ts <= 100; ts++ {
		entries = append(entries, entry(ts, "WinEvent:MsiInstaller", "Error 1704"))
	}
	for ts := int64(101); ts <= 105; ts++ {
		entries = append(entries, entry(ts, "journald:sshd", fmt.Sprintf("unique-%d", ts)))
	}

	got := finalize(entries, protocol.LogRequest{}, 10)

	if len(got) != 6 {
		t.Fatalf("got %d entries, want 6 (one collapsed run plus five distinct)", len(got))
	}

	var found int
	for _, e := range got {
		if strings.HasPrefix(e.Message, "unique-") {
			found++
		}
	}
	if found != 5 {
		t.Errorf("kept %d distinct entries, want 5 — the limit cut what collapsing had made room for", found)
	}
}

func TestFinalize_Sorts(t *testing.T) {
	entries := []protocol.LogEntry{
		entry(30, "s", "c"),
		entry(10, "s", "a"),
		entry(20, "s", "b"),
	}

	got := timestamps(finalize(entries, protocol.LogRequest{}, 100))

	want := []int64{10, 20, 30}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("timestamps = %v, want %v", got, want)
		}
	}
}

func TestFinalize_AppliesWindow(t *testing.T) {
	entries := []protocol.LogEntry{
		entry(10, "s", "too old"),
		entry(50, "s", "inside"),
		entry(90, "s", "too new"),
	}

	got := finalize(entries, protocol.LogRequest{Since: 20, Until: 80}, 100)

	if len(got) != 1 {
		t.Fatalf("got %d entries %v, want 1", len(got), timestamps(got))
	}
	if got[0].Message != "inside" {
		t.Errorf("kept %q, want the entry inside the window", got[0].Message)
	}
}

// TestFinalize_ZeroBoundsAreUnset pins that a zero Since/Until means unbounded
// rather than the epoch, which is what lets an older server send only MinLevel
// and get the previous behavior.
func TestFinalize_ZeroBoundsAreUnset(t *testing.T) {
	entries := []protocol.LogEntry{
		entry(10, "s", "a"),
		entry(50, "s", "b"),
		entry(90, "s", "c"),
	}

	if got := finalize(entries, protocol.LogRequest{}, 100); len(got) != 3 {
		t.Errorf("got %d entries, want 3 — zero bounds must not filter", len(got))
	}
}

func TestFinalize_EmptyIsNotNil(t *testing.T) {
	// A nil slice marshals as JSON null; the UI expects an array.
	got := finalize(nil, protocol.LogRequest{}, 100)
	if got == nil {
		t.Fatal("finalize returned nil, want an empty slice")
	}
	b, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(b) != "[]" {
		t.Errorf("marshalled as %s, want []", b)
	}
}

func TestGatherLimit(t *testing.T) {
	tests := []struct {
		name    string
		limit   int
		maxLogs int
		want    int
	}{
		{"unset falls back to the platform cap", 0, 10000, 10000},
		{"smaller request wins", 500, 10000, 500},
		{"larger request is clamped", 90000, 10000, 10000},
		{"negative is treated as unset", -1, 10000, 10000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := gatherLimit(protocol.LogRequest{Limit: tt.limit}, tt.maxLogs)
			if got != tt.want {
				t.Errorf("gatherLimit(%d, %d) = %d, want %d", tt.limit, tt.maxLogs, got, tt.want)
			}
		})
	}
}

func TestInWindow(t *testing.T) {
	tests := []struct {
		name  string
		ts    int64
		since int64
		until int64
		want  bool
	}{
		{"no bounds", 50, 0, 0, true},
		{"inside both", 50, 20, 80, true},
		{"below since", 10, 20, 80, false},
		{"above until", 90, 20, 80, false},
		{"on since boundary", 20, 20, 80, true},
		{"on until boundary", 80, 20, 80, true},
		{"zero timestamp with no bounds", 0, 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inWindow(tt.ts, protocol.LogRequest{Since: tt.since, Until: tt.until})
			if got != tt.want {
				t.Errorf("inWindow(%d, since=%d, until=%d) = %v, want %v",
					tt.ts, tt.since, tt.until, got, tt.want)
			}
		})
	}
}

// TestLevelToPriority_Ordering walks the levels from most to least severe and
// requires the priority to increase at every step.
func TestLevelToPriority_Ordering(t *testing.T) {
	ordered := []protocol.LogLevel{
		protocol.LevelEmergency,
		protocol.LevelAlert,
		protocol.LevelCritical,
		protocol.LevelError,
		protocol.LevelWarning,
		protocol.LevelNotice,
		protocol.LevelInfo,
		protocol.LevelDebug,
	}

	for i := 1; i < len(ordered); i++ {
		prev, cur := ordered[i-1], ordered[i]
		if levelToPriority(prev) >= levelToPriority(cur) {
			t.Errorf("%s (%d) should be more severe than %s (%d)",
				prev, levelToPriority(prev), cur, levelToPriority(cur))
		}
	}
}

// TestLevelToPriority_DisagreesWithStringOrder is the regression guard for the
// FreeBSD source-selection bug. protocol.LogLevel is a string type, so comparing
// two levels with < or <= compares them alphabetically. These are the pairs
// where that disagrees with severity, and where the direction of the resulting
// bug flipped: at ERROR every source was read and nothing filtered, at WARNING
// every source was skipped and the result was empty.
func TestLevelToPriority_DisagreesWithStringOrder(t *testing.T) {
	pairs := []struct {
		severe, mild protocol.LogLevel
	}{
		{protocol.LevelError, protocol.LevelNotice},
		{protocol.LevelError, protocol.LevelWarning},
		{protocol.LevelCritical, protocol.LevelNotice},
		{protocol.LevelAlert, protocol.LevelNotice},
		{protocol.LevelEmergency, protocol.LevelNotice},
		{protocol.LevelCritical, protocol.LevelInfo},
	}

	for _, p := range pairs {
		if levelToPriority(p.severe) >= levelToPriority(p.mild) {
			t.Errorf("%s must outrank %s by priority", p.severe, p.mild)
		}
		if p.severe < p.mild {
			// Not a failure, but the reason this function has to exist: the
			// lexical order says the opposite of the severity order here.
			t.Logf("lexical order disagrees as expected: %q < %q", p.severe, p.mild)
		}
	}
}
