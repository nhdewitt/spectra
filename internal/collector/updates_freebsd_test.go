//go:build freebsd

package collector

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

const pkgUpgradeOutput = `Checking for upgrades (102 candidates): 100%
Processing candidates (102 candidates): 100%
The following 22 package(s) will be affected (of 0 checked):

New packages to be INSTALLED:
	libada: 3.4.2 [FreeBSD-ports]
	libebur128: 1.2.6 [FreeBSD-ports]
	llvm19-lite: 19.1.7_1 [FreeBSD-ports]
	uvwasi: 0.0.23 [FreeBSD-ports]

Installed packages to be UPGRADED:
	firefox: 147.0.4,2 -> 148.0,2 [FreeBSD-ports]
	libmtp: 1.1.22 -> 1.1.23 [FreeBSD-ports]
	libsoup3: 3.6.5_2 -> 3.6.6 [FreeBSD-ports]
	nss: 3.120 -> 3.120.1 [FreeBSD-ports]
	py311-trio: 0.32.0 -> 0.33.0 [FreeBSD-ports]
	smartmontools: 7.5_1 -> 7.5_2 [FreeBSD-ports]
	xterm: 406 -> 407 [FreeBSD-ports]

Number of packages to be installed: 15
Number of packages to be upgraded: 7
The process will require 515 MiB more space.
101 MiB to be downloaded.
`

func TestParsePkgUpgrade(t *testing.T) {
	updates, err := parsePkgUpgrade(strings.NewReader(pkgUpgradeOutput))
	if err != nil {
		t.Fatal(err)
	}

	if len(updates) != 7 {
		t.Fatalf("got %d updates, want 7", len(updates))
	}
}

func TestParsePkgUpgradeNames(t *testing.T) {
	updates, err := parsePkgUpgrade(strings.NewReader(pkgUpgradeOutput))
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]string{
		"firefox":       "148.0,2",
		"libmtp":        "1.1.23",
		"libsoup3":      "3.6.6",
		"nss":           "3.120.1",
		"py311-trio":    "0.33.0",
		"smartmontools": "7.5_2",
		"xterm":         "407",
	}

	for _, u := range updates {
		expectedVer, ok := want[u.Name]
		if !ok {
			t.Errorf("unexpected package: %q", u.Name)
			continue
		}
		if u.Version != expectedVer {
			t.Errorf("%s: version = %q, want %q", u.Name, u.Version, expectedVer)
		}
		delete(want, u.Name)
	}

	for name := range want {
		t.Errorf("missing package: %q", name)
	}
}

func TestParsePkgUpgradeNoInstalled(t *testing.T) {
	updates, err := parsePkgUpgrade(strings.NewReader(pkgUpgradeOutput))
	if err != nil {
		t.Fatal(err)
	}

	for _, u := range updates {
		if u.Name == "libada" || u.Name == "uvwasi" || u.Name == "llvm19-lite" {
			t.Errorf("INSTALLED package %q should not be in updates", u.Name)
		}
	}
}

func TestParsePkgUpgradeNoUpdates(t *testing.T) {
	input := `Checking for upgrades (50 candidates): 100%
Processing candidates (50 candidates): 100%
Your packages are up to date.
`
	updates, err := parsePkgUpgrade(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(updates) != 0 {
		t.Errorf("got %d updates, want 0", len(updates))
	}
}

func TestParsePkgUpgradeEmpty(t *testing.T) {
	updates, err := parsePkgUpgrade(strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	if len(updates) != 0 {
		t.Errorf("got %d updates, want 0", len(updates))
	}
}

func TestParsePkgUpgradeLine(t *testing.T) {
	tests := []struct {
		name        string
		line        string
		wantName    string
		wantVersion string
		wantOk      bool
	}{
		{"normal", "firefox: 147.0.4,2 -> 148.0,2 [FreeBSD-ports]", "firefox", "148.0,2", true},
		{"underscore version", "smartmontools: 7.5_1 -> 7.5_2 [FreeBSD-ports]", "smartmontools", "7.5_2", true},
		{"simple", "xterm: 406 -> 407 [FreeBSD-ports]", "xterm", "407", true},
		{"no arrow", "libada: 3.4.2 [FreeBSD-ports]", "", "", false},
		{"empty", "", "", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			u, ok := parsePkgUpgradeLine(tc.line)
			if ok != tc.wantOk {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOk)
			}
			if !ok {
				return
			}
			if u.Name != tc.wantName {
				t.Errorf("name = %q, want %q", u.Name, tc.wantName)
			}
			if u.Version != tc.wantVersion {
				t.Errorf("version = %q, want %q", u.Version, tc.wantVersion)
			}
		})
	}
}

func TestCollectUpdatesIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	metrics, err := CollectUpdates(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(metrics) != 1 {
		t.Fatalf("got %d metrics, want 1", len(metrics))
	}

	um, ok := metrics[0].(protocol.UpdateMetric)
	if !ok {
		t.Fatalf("metric type = %T, want UpdateMetric", metrics[0])
	}

	t.Logf("package manager: %s", um.PackageManager)
	t.Logf("pending: %d, security: %d, reboot: %v", um.PendingCount, um.SecurityCount, um.RebootRequired)

	if um.PackageManager != "pkg" {
		t.Errorf("package manager = %q, want pkg", um.PackageManager)
	}

	if um.PendingCount != len(um.Packages) {
		t.Errorf("PendingCount (%d) != len(Packages) (%d)", um.PendingCount, len(um.Packages))
	}

	// Log all pending updates
	if len(um.Packages) > 0 {
		t.Log("--- pending updates ---")
		for _, p := range um.Packages {
			t.Logf("  %s -> %s", p.Name, p.Version)
		}
	} else {
		t.Log("system is up to date (0 pending)")
	}

	// Every package should have a name and version
	for i, p := range um.Packages {
		if p.Name == "" {
			t.Errorf("packages[%d] has empty name", i)
		}
		if p.Version == "" {
			t.Errorf("packages[%d] (%s) has empty version", i, p.Name)
		}
	}
}
