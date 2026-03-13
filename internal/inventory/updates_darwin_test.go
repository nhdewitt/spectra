//go:build darwin

package inventory

import (
	"context"
	"testing"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func TestParseSoftwareUpdate_WithUpdates(t *testing.T) {
	data := []byte(`Software Update found the following new or updated software:
* Label: macOS Ventura 13.6.1
  Title: macOS Ventura 13.6.1, Version: 13.6.1, Size: 1024K, Recommended: YES, Action: restart,
* Label: Security Update 2024-001
  Title: Security Update 2024-001, Version: 1.0, Size: 512K, Recommended: YES,
* Label: XProtect Update
  Title: XProtect Update, Version: 2199, Size: 128K, Recommended: YES,
`)

	updates, reboot := parseSoftwareUpdate(data)

	if len(updates) != 3 {
		t.Fatalf("got %d updates, want 3", len(updates))
	}
	if !reboot {
		t.Error("expected rebootRequired=true")
	}
	if updates[0].Name != "macOS Ventura" {
		t.Errorf("updates[0].Name = %q", updates[0].Name)
	}
	if updates[0].Version != "13.6.1" {
		t.Errorf("updates[0].Version = %q", updates[0].Version)
	}
	if updates[1].Name != "Security Update 2024-001" {
		t.Errorf("updates[1].Name = %q", updates[1].Name)
	}
	if updates[1].Version != "1.0" {
		t.Errorf("updates[1].Version = %q", updates[1].Version)
	}
	if updates[2].Name != "XProtect Update" {
		t.Errorf("updates[2].Name = %q", updates[2].Name)
	}
	if updates[2].Version != "2199" {
		t.Errorf("updats[2].Version = %q", updates[2].Version)
	}
}

func TestParseSoftwareUpdate_NoUpdates(t *testing.T) {
	data := []byte("No new software available.\n")
	updates, reboot := parseSoftwareUpdate(data)
	if len(updates) != 0 {
		t.Errorf("got %d updates, want 0", len(updates))
	}
	if reboot {
		t.Error("expected rebootRequired=false")
	}
}

func TestParseSOftwareUpdate_Empty(t *testing.T) {
	updates, _ := parseSoftwareUpdate(nil)
	if len(updates) != 0 {
		t.Errorf("got %d updates, expected 0", len(updates))
	}
}

func TestExtractField(t *testing.T) {
	line := "Title: macOS Ventura 13.6.1, Version: 13.6.1, Size: 1024K, Recommended: YES,"

	tests := []struct {
		key, want string
	}{
		{"Version:", "13.6.1"},
		{"Size:", "1024K"},
		{"Recommended:", "YES"},
		{"Missing:", ""},
	}

	for _, tt := range tests {
		got := extractField(line, tt.key)
		if got != tt.want {
			t.Errorf("extractField(%q) = %q, want %q", tt.key, got, tt.want)
		}
	}
}

func TestGetUpdates_Integration(t *testing.T) {
	metric, err := GetUpdates(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updates := metric[0].(protocol.UpdateMetric)

	t.Logf("Found %d updates", updates.PendingCount)
	for _, update := range updates.Packages {
		t.Logf("Name: %s Version: %s", update.Name, update.Version)
	}
}
