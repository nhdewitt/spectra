package version

import (
	"strings"
	"testing"
)

func TestDefaults(t *testing.T) {
	if Version != "dev" {
		t.Errorf("expected default Version = 'dev', got %q", Version)
	}
	if Commit != "unknown" {
		t.Errorf("expected default Commit = 'unknown', got %q", Commit)
	}
	if Date != "unknown" {
		t.Errorf("expected default Date = 'unknown', got %q", Date)
	}
}

func TestFull(t *testing.T) {
	result := Full()

	if !strings.Contains(result, Version) {
		t.Errorf("Full() = %q, should contain Version %q", result, Version)
	}
	if !strings.Contains(result, Commit) {
		t.Errorf("Full() = %q, should contain Commit %q", result, Commit)
	}
	if !strings.Contains(result, Date) {
		t.Errorf("Full() = %q, should contain Date %q", result, Date)
	}
}

func TestFull_Format(t *testing.T) {
	origVersion, origCommit, origDate := Version, Commit, Date
	defer func() {
		Version, Commit, Date = origVersion, origCommit, origDate
	}()

	Version = "1.2.3"
	Commit = "abc1234"
	Date = "2026-04-12T00:00:00Z"

	expected := "1.2.3 (abc1234) 2026-04-12T00:00:00Z"
	if got := Full(); got != expected {
		t.Errorf("Full() = %q, want %q", got, expected)
	}
}

func TestUserAgent(t *testing.T) {
	tests := []struct {
		component string
		want      string
	}{
		{"Agent", "Spectra-Agent/" + Version},
		{"Server", "Spectra-Server/" + Version},
		{"", "Spectra-/" + Version},
	}

	for _, tt := range tests {
		t.Run(tt.component, func(t *testing.T) {
			if got := UserAgent(tt.component); got != tt.want {
				t.Errorf("UserAgent(%q) = %q, want %q", tt.component, got, tt.want)
			}
		})
	}
}

func TestUserAgent_WithCustomVersion(t *testing.T) {
	origVersion := Version
	defer func() { Version = origVersion }()

	Version = "0.5.0"

	if got := UserAgent("Agent"); got != "Spectra-Agent/0.5.0" {
		t.Errorf("UserAgent('Agent') = %q, want 'Spectra-Agent/0.5.0'", got)
	}
}
