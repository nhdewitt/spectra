//go:build linux

package collector

import (
	"strings"
	"testing"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func TestParseAptLine(t *testing.T) {
	tests := []struct {
		name, line string
		want       protocol.PendingUpdate
		ok         bool
	}{
		{
			name: "Standard package",
			line: "vim/jammy-updates 2:9.0.1000-4ubuntu2 amd64 [upgradable from: 2:8.2.3995-1ubuntu2.13]",
			want: protocol.PendingUpdate{Name: "vim", Version: "2:9.0.1000-4ubuntu2", Security: false},
			ok:   true,
		},
		{
			name: "Security package",
			line: "openssl/jammy-security 3.0.2-0ubuntu1.12 amd64 [upgradable from: 3.0.2-0ubuntu1.10]",
			want: protocol.PendingUpdate{Name: "openssl", Version: "3.0.2-0ubuntu1.12", Security: true},
			ok:   true,
		},
		{
			name: "Header line",
			line: "Listing...",
			ok:   false,
		},
		{
			name: "Empty line",
			line: "",
			ok:   false,
		},
		{
			name: "Whitespace only",
			line: "   ",
			ok:   false,
		},
		{
			name: "No slash",
			line: "some random text without a slash",
			ok:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseAptLine(tt.line)
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v", ok, tt.ok)
			}
			if !ok {
				return
			}
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseApkLine(t *testing.T) {
	tests := []struct {
		name, line string
		want       protocol.PendingUpdate
		ok         bool
	}{
		{
			name: "Standard upgrade",
			line: "(1/3) Upgrading musl (1.2.4-r1 -> 1.2.4-r2)",
			want: protocol.PendingUpdate{Name: "musl", Version: "1.2.4-r2"},
			ok:   true,
		},
		{
			name: "Large index",
			line: "(12/99) Upgrading openssl (3.1.4-r0 -> 3.1.4-r1)",
			want: protocol.PendingUpdate{Name: "openssl", Version: "3.1.4-r1"},
			ok:   true,
		},
		{
			name: "Summary line",
			line: "OK: 45 MiB in 78 packages",
			ok:   false,
		},
		{
			name: "Empty line",
			line: "",
			ok:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseApkLine(tt.line)
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v", ok, tt.ok)
			}
			if !ok {
				return
			}
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParsePacmanLine(t *testing.T) {
	tests := []struct {
		name, line string
		want       protocol.PendingUpdate
		ok         bool
	}{
		{
			name: "Standard update",
			line: "linux 6.7.4.arch1-1 -> 6.7.5.arch1-1",
			want: protocol.PendingUpdate{Name: "linux", Version: "6.7.5.arch1-1"},
			ok:   true,
		},
		{
			name: "Package with hyphen",
			line: "openssl 3.2.0-1 -> 3.2.1-1",
			want: protocol.PendingUpdate{Name: "openssl", Version: "3.2.1-1"},
			ok:   true,
		},
		{
			name: "Malformed - no arrow",
			line: "linux 6.7.4.arch1-1 6.7.5.arch1-1",
			ok:   false,
		},
		{
			name: "Too few fields",
			line: "linux",
			ok:   false,
		},
		{
			name: "Empty line",
			line: "",
			ok:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parsePacmanLine(tt.line)
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v", ok, tt.ok)
			}
			if !ok {
				return
			}
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestExtractRPMName(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"openssl-libs-1:3.0.7-25.el9_3.x86_64", "openssl-libs"},
		{"kernel-5.14.0-362.24.1.el9_3.x86_64", "kernel"},
		{"vim-common-2:9.0.2081-1.el9.x86_64", "vim-common"},
		{"bash-5.2.15-3.el9.x86_64", "bash"},
		{"python3-libs-3.9.18-1.el9_3.1.x86_64", "python3-libs"},
		// No arch suffix
		{"openssl-libs-1:3.0.7-25.el9_3", "openssl-libs"},
		// Single component name
		{"glibc-2.34-60.el9.x86_64", "glibc"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := extractRPMName(tt.input)
			if got != tt.want {
				t.Errorf("extractRPMName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestScanUpdates_Apt(t *testing.T) {
	input := `Listing...
vim/jammy-updates 2:9.0.1000-4ubuntu2 amd64 [upgradable from: 2:8.2.3995-1ubuntu2.13]
openssl/jammy-security 3.0.2-0ubuntu1.12 amd64 [upgradable from: 3.0.2-0ubuntu1.10]
curl/jammy-updates 7.81.0-1ubuntu1.15 amd64 [upgradable from: 7.81.0-1ubuntu1.14]
`
	updates, err := scanUpdates(strings.NewReader(input), parseAptLine)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(updates) != 3 {
		t.Fatalf("count: got %d, want 3", len(updates))
	}

	if updates[0].Name != "vim" || updates[0].Security {
		t.Errorf("updates[0]: got %+v", updates[0])
	}
	if updates[1].Name != "openssl" || !updates[1].Security {
		t.Errorf("updates[1]: got %+v", updates[1])
	}
}

func TestScanUpdates_Apk(t *testing.T) {
	input := `(1/3) Upgrading musl (1.2.4-r1 -> 1.2.4-r2)
(2/3) Upgrading busybox (1.36.1-r2 -> 1.36.1-r3)
(3/3) Upgrading openssl (3.1.4-r0 -> 3.1.4-r1)
OK: 45 MiB in 78 packages
`
	updates, err := scanUpdates(strings.NewReader(input), parseApkLine)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(updates) != 3 {
		t.Fatalf("count: got %d, want 3", len(updates))
	}
	if updates[0].Name != "musl" || updates[0].Version != "1.2.4-r2" {
		t.Errorf("updates[0]: got %+v", updates[0])
	}
}

func TestScanUpdates_Pacman(t *testing.T) {
	input := `linux 6.7.4.arch1-1 -> 6.7.5.arch1-1
openssl 3.2.0-1 -> 3.2.1-1
this is not a valid line
vim 9.0.2190-1 -> 9.1.0-1
`
	updates, err := scanUpdates(strings.NewReader(input), parsePacmanLine)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(updates) != 3 {
		t.Fatalf("count: got %d, want 3", len(updates))
	}
}

func TestScanUpdates_Empty(t *testing.T) {
	updates, err := scanUpdates(strings.NewReader(""), parseAptLine)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(updates) != 0 {
		t.Errorf("count: got %d, want 0", len(updates))
	}
}

func TestScanUpdates_AllSkipped(t *testing.T) {
	input := `Listing...

`
	updates, err := scanUpdates(strings.NewReader(input), parseAptLine)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(updates) != 0 {
		t.Errorf("count: got %d, want 0", len(updates))
	}
}
