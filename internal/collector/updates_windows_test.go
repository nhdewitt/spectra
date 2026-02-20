//go:build windows

package collector

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func TestParseWindowsUpdates(t *testing.T) {
	input := `2024-01 Cumulative Update for Windows 11|KB5034441|true
Servicing Stack Update|KB5034439|false
2024-01 Security Update for .NET|KB5034442,KB5034443|true
`
	updates, err := parseWindowsUpdates(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(updates) != 3 {
		t.Fatalf("count: got %d, want 3", len(updates))
	}

	// Security cumulative update
	if updates[0].Name != "2024-01 Cumulative Update for Windows 11" {
		t.Errorf("updates[0].Name: got %q", updates[0].Name)
	}
	if updates[0].Version != "KB5034441" {
		t.Errorf("updates[0].Version: got %q", updates[0].Version)
	}
	if !updates[0].Security {
		t.Error("updates[0] should be security")
	}

	// Non-security update
	if updates[1].Name != "Servicing Stack Update" {
		t.Errorf("updates[1].Name: got %q", updates[1].Name)
	}
	if updates[1].Security {
		t.Error("updates[1] should not be security")
	}

	// Multiple KB IDs
	if updates[2].Version != "KB5034442,KB5034443" {
		t.Errorf("updates[2].Version: got %q", updates[2].Version)
	}
	if !updates[2].Security {
		t.Error("updates[2] should be security")
	}
}

func TestParseWindowsUpdates_Empty(t *testing.T) {
	updates, err := parseWindowsUpdates(strings.NewReader(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(updates) != 0 {
		t.Errorf("count: got %d, want 0", len(updates))
	}
}

func TestParseWindowsUpdates_BlankLines(t *testing.T) {
	input := `
2024-01 Cumulative Update|KB5034441|true

Servicing Stack Update|KB5034439|false

`
	updates, err := parseWindowsUpdates(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(updates) != 2 {
		t.Fatalf("count: got %d, want 2", len(updates))
	}
}

func TestParseWindowsUpdates_MalformedLines(t *testing.T) {
	input := `2024-01 Cumulative Update|KB5034441|true
this has no pipes
only|two fields
2024-01 Security Update|KB5034442|false
`
	updates, err := parseWindowsUpdates(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(updates) != 2 {
		t.Fatalf("count: got %d, want 2 (malformed skipped)", len(updates))
	}
}

func TestParseWindowsUpdates_InvalidBool(t *testing.T) {
	input := `Some Update|KB1234|notabool
`
	updates, err := parseWindowsUpdates(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(updates) != 1 {
		t.Fatalf("count: got %d, want 1", len(updates))
	}
	// strconv.ParseBool returns false on error
	if updates[0].Security {
		t.Error("invalid bool should default to false")
	}
}

func TestParseWindowsUpdates_PipesInTitle(t *testing.T) {
	// SplitN with n=3 ensures pipes in the title don't break parsing
	// but title is the first field so this tests pipes in the bool field area
	input := `Update with extra|data|in|title|KB1234|true
`
	updates, err := parseWindowsUpdates(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// SplitN(line, "|", 3) gives ["Update with extra", "data", "in|title|KB1234|true"]
	// strconv.ParseBool("in|title|KB1234|true") fails -> false
	if len(updates) != 1 {
		t.Fatalf("count: got %d, want 1", len(updates))
	}
	if updates[0].Security {
		t.Error("malformed security field should default to false")
	}
}

func TestFindPowerShell(t *testing.T) {
	ps := findPowerShell()
	if ps == "" {
		t.Skip("no powershell available")
	}

	// Verify the returned path is executable
	if _, err := exec.LookPath(ps); err != nil {
		t.Errorf("findPowerShell returned %q but it's not executable: %v", ps, err)
	}

	t.Logf("found powershell: %s", ps)
}

func TestCollectUpdates_Integration(t *testing.T) {
	if findPowerShell() == "" {
		t.Skip("no powershell available")
	}

	ctx := t.Context()
	metrics, err := CollectUpdates(ctx)
	if err != nil {
		t.Fatalf("CollectUpdates failed: %v", err)
	}

	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(metrics))
	}

	um, ok := metrics[0].(protocol.UpdateMetric)
	if !ok {
		t.Fatalf("expected UpdateMetric, got %T", metrics[0])
	}

	t.Logf("Pending: %d, Security: %d, Reboot: %v, Manager: %s",
		um.PendingCount, um.SecurityCount, um.RebootRequired, um.PackageManager)

	if um.PackageManager != "windows_update" {
		t.Errorf("PackageManager: got %q, want %q", um.PackageManager, "windows_update")
	}
	if um.SecurityCount > um.PendingCount {
		t.Errorf("SecurityCount (%d) > PendingCount (%d)", um.SecurityCount, um.PendingCount)
	}
}
