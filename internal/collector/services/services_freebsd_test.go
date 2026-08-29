//go:build freebsd

package services

import (
	"bufio"
	"errors"
	"strings"
	"testing"
)

func TestParseEnabled(t *testing.T) {
	// "service -e" emits full rc.d script paths; only the basename is kept.
	input := `/etc/rc.d/sshd
/etc/rc.d/syslogd
/usr/local/etc/rc.d/nginx
`
	set, err := parseEnabled(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, want := range []string{"sshd", "syslogd", "nginx"} {
		if !set[want] {
			t.Errorf("expected %q in the enabled set", want)
		}
	}
	if len(set) != 3 {
		t.Errorf("got %d entries, want 3", len(set))
	}
}

func TestParseEnabled_SkipsBlankLines(t *testing.T) {
	input := "/etc/rc.d/sshd\n\n   \n/etc/rc.d/cron\n"

	set, err := parseEnabled(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(set) != 2 {
		t.Errorf("got %d entries, want 2: %v", len(set), set)
	}
	if set[""] {
		t.Error("a blank line produced an empty service name")
	}
}

func TestParseEnabled_Empty(t *testing.T) {
	set, err := parseEnabled(strings.NewReader(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(set) != 0 {
		t.Errorf("got %d entries, want 0", len(set))
	}
}

func TestParseAll(t *testing.T) {
	input := "sshd\nsyslogd\nnginx\ncron\n"

	names, err := parseAll(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 4 {
		t.Fatalf("got %d names, want 4: %v", len(names), names)
	}
	if names[0] != "sshd" || names[3] != "cron" {
		t.Errorf("order not preserved: %v", names)
	}
}

func TestParseAll_TrimsWhitespace(t *testing.T) {
	input := "  sshd  \n\tsyslogd\t\n\n"

	names, err := parseAll(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 2 {
		t.Fatalf("got %d names, want 2: %v", len(names), names)
	}
	for _, n := range names {
		if n != strings.TrimSpace(n) {
			t.Errorf("name %q was not trimmed", n)
		}
	}
}

func TestParseAll_Empty(t *testing.T) {
	names, err := parseAll(strings.NewReader(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 0 {
		t.Errorf("got %d names, want 0: %v", len(names), names)
	}
}

// The readers are bytes.Readers over buffered exec output, so ErrTooLong is
// the only error a scanner can produce here. Dropping it would silently
// truncate the service list -- the agent would report a short list with no
// indication why.
func TestParseAll_LineTooLongIsReturned(t *testing.T) {
	input := strings.Repeat("x", bufio.MaxScanTokenSize+1) + "\n"

	names, err := parseAll(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected an error for an oversized line, got nil")
	}
	if !errors.Is(err, bufio.ErrTooLong) {
		t.Errorf("expected bufio.ErrTooLong, got %v", err)
	}
	if names != nil {
		t.Errorf("expected a nil slice alongside the error, got %v", names)
	}
}

func TestParseEnabled_LineTooLongIsReturned(t *testing.T) {
	input := strings.Repeat("x", bufio.MaxScanTokenSize+1) + "\n"

	set, err := parseEnabled(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected an error for an oversized line, got nil")
	}
	if !errors.Is(err, bufio.ErrTooLong) {
		t.Errorf("expected bufio.ErrTooLong, got %v", err)
	}
	if set != nil {
		t.Errorf("expected a nil map alongside the error, got %v", set)
	}
}

// A truncated list is worse than no list, so an oversized line aborts rather
// than reporting the services parsed before it.
func TestParseAll_TruncationIsNotPartial(t *testing.T) {
	input := "sshd\nsyslogd\n" + strings.Repeat("x", bufio.MaxScanTokenSize+1) + "\ncron\n"

	names, err := parseAll(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if names != nil {
		t.Errorf("expected no partial results, got %v", names)
	}
}

func TestIsEnabled(t *testing.T) {
	set := map[string]bool{"sshd": true}

	loadState, status := isEnabled("sshd", set)
	if loadState != "enabled" || status != "active" {
		t.Errorf("enabled service: got (%q, %q), want (\"enabled\", \"active\")", loadState, status)
	}

	loadState, status = isEnabled("nginx", set)
	if loadState != "disabled" || status != "inactive" {
		t.Errorf("disabled service: got (%q, %q), want (\"disabled\", \"inactive\")", loadState, status)
	}
}
