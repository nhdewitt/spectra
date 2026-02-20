//go:build linux || freebsd

package collector

import (
	"fmt"
	"strings"
	"testing"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func TestParseGenericOutput_Dpkg(t *testing.T) {
	input := `adduser,3.118ubuntu5,Ubuntu Core Developers <ubuntu-devel-discuss@lists.ubuntu.com>
apt,2.0.9,APT Development Team <deity@lists.debian.org>
base-files,11ubuntu5.6,Santiago Vila <sanvila@debian.org>
`
	reader := strings.NewReader(strings.TrimSpace(input))
	apps, err := parseGenericOutput(reader, ",", "deb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(apps) != 3 {
		t.Fatalf("expected 3 apps, got %d", len(apps))
	}
	if apps[0].Name != "adduser" || apps[0].Version != "3.118ubuntu5" {
		t.Errorf("unexpected first app: %+v", apps[0])
	}
	// cleanVendor should strip the email
	if apps[0].Vendor == "" {
		t.Error("expected non-empty vendor after cleanVendor")
	}
}

func TestParseGenericOutput_Rpm(t *testing.T) {
	input := `bash,5.1.8,Red Hat, Inc.
kernel,5.14.0,Red Hat, Inc.
`
	reader := strings.NewReader(strings.TrimSpace(input))
	apps, err := parseGenericOutput(reader, ",", "rpm")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// "bash,5.1.8,Red Hat, Inc." splits into ["bash", "5.1.8", "Red Hat, Inc."] with SplitN(,3)
	if len(apps) != 2 {
		t.Fatalf("expected 2 apps, got %d", len(apps))
	}
	if apps[0].Name != "bash" || apps[0].Version != "5.1.8" {
		t.Errorf("unexpected first app: %+v", apps[0])
	}
}

func TestParseGenericOutput_Pacman(t *testing.T) {
	input := `bash 5.2.026-2
coreutils 9.4-3
linux 6.7.4.arch1-1
`
	reader := strings.NewReader(strings.TrimSpace(input))
	apps, err := parseGenericOutput(reader, " ", "arch")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(apps) != 3 {
		t.Fatalf("expected 3 apps, got %d", len(apps))
	}
	if apps[2].Name != "linux" || apps[2].Version != "6.7.4.arch1-1" {
		t.Errorf("unexpected third app: %+v", apps[2])
	}
	if apps[0].Vendor != "arch" {
		t.Errorf("expected vendor 'arch', got %q", apps[0].Vendor)
	}
}

func TestParseGenericOutput_Apk(t *testing.T) {
	input := `musl-1.2.4-r2
busybox-1.36.1-r5
alpine-baselayout-3.4.3-r1
`
	reader := strings.NewReader(strings.TrimSpace(input))
	apps, err := parseGenericOutput(reader, "-", "alpine")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(apps) != 3 {
		t.Fatalf("expected 3 apps, got %d", len(apps))
	}
	if apps[0].Name != "musl" || apps[0].Version != "1.2.4-r2" {
		t.Errorf("unexpected first app: %+v", apps[0])
	}
	if apps[1].Name != "busybox" || apps[1].Version != "1.36.1-r5" {
		t.Errorf("unexpected second app: %+v", apps[1])
	}
}

func TestParseGenericOutput_Pkg(t *testing.T) {
	input := `bash,5.2.37,ports@FreeBSD.org
curl,8.11.1,jhixson@FreeBSD.org
go,1.23.4,jlaffaye@FreeBSD.org
nginx,1.26.2,vanilla@FreeBSD.org
python311,3.11.11,antoine@FreeBSD.org
`
	reader := strings.NewReader(strings.TrimSpace(input))
	apps, err := parseGenericOutput(reader, ",", "freebsd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(apps) != 5 {
		t.Fatalf("expected 5 apps, got %d", len(apps))
	}

	expected := []struct {
		name    string
		version string
	}{
		{"bash", "5.2.37"},
		{"curl", "8.11.1"},
		{"go", "1.23.4"},
		{"nginx", "1.26.2"},
		{"python311", "3.11.11"},
	}

	for i, want := range expected {
		if apps[i].Name != want.name {
			t.Errorf("app[%d] name = %q, want %q", i, apps[i].Name, want.name)
		}
		if apps[i].Version != want.version {
			t.Errorf("app[%d] version = %q, want %q", i, apps[i].Version, want.version)
		}
	}
}

func TestParseGenericOutput_PkgMaintainerCleaned(t *testing.T) {
	input := `pkg,1.21.3,pkg@FreeBSD.org`
	reader := strings.NewReader(input)
	apps, err := parseGenericOutput(reader, ",", "freebsd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(apps) != 1 {
		t.Fatalf("expected 1 app, got %d", len(apps))
	}
	// cleanVendor strips emails, so bare email -> empty -> falls back to defaultVendor
	if apps[0].Vendor != "freebsd" {
		t.Errorf("vendor = %q, want %q (bare email should fall back to default)", apps[0].Vendor, "freebsd")
	}
}

func TestParseGenericOutput_PkgNameWithNumbers(t *testing.T) {
	// FreeBSD ports often have version numbers in package names
	input := `python311,3.11.11,antoine@FreeBSD.org
py311-pip,24.3.1,antoine@FreeBSD.org
rust-cbindgen,0.27.0,gecko@FreeBSD.org
p5-Net-SSLeay,1.94,perl@FreeBSD.org
`
	reader := strings.NewReader(strings.TrimSpace(input))
	apps, err := parseGenericOutput(reader, ",", "freebsd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(apps) != 4 {
		t.Fatalf("expected 4 apps, got %d", len(apps))
	}
	if apps[0].Name != "python311" {
		t.Errorf("expected 'python311', got %q", apps[0].Name)
	}
	if apps[1].Name != "py311-pip" {
		t.Errorf("expected 'py311-pip', got %q", apps[1].Name)
	}
	if apps[3].Name != "p5-Net-SSLeay" {
		t.Errorf("expected 'p5-Net-SSLeay', got %q", apps[3].Name)
	}
}

func TestParseGenericOutput_EmptyInput(t *testing.T) {
	reader := strings.NewReader("")
	apps, err := parseGenericOutput(reader, ",", "deb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(apps) != 0 {
		t.Errorf("expected 0 apps, got %d", len(apps))
	}
}

func TestParseGenericOutput_OnlyWhitespace(t *testing.T) {
	reader := strings.NewReader("   \n\t\n   \n")
	apps, err := parseGenericOutput(reader, ",", "deb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(apps) != 0 {
		t.Errorf("expected 0 apps, got %d", len(apps))
	}
}

func TestParseGenericOutput_MalformedLines(t *testing.T) {
	input := `valid-package,1.0,Vendor
invalidline
another-valid,2.0,Other Vendor
,,,
just-name
`
	reader := strings.NewReader(strings.TrimSpace(input))
	apps, err := parseGenericOutput(reader, ",", "deb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(apps) != 2 {
		t.Errorf("expected 2 valid apps, got %d", len(apps))
	}
}

func TestParseGenericOutput_SpecialCharactersInNames(t *testing.T) {
	input := `lib++extra,1.0,Vendor
python3.11,3.11.4,Python Foundation
c#-compiler,5.0,Microsoft
`
	reader := strings.NewReader(strings.TrimSpace(input))
	apps, err := parseGenericOutput(reader, ",", "deb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(apps) != 3 {
		t.Fatalf("expected 3 apps, got %d", len(apps))
	}
	if apps[0].Name != "lib++extra" {
		t.Errorf("expected 'lib++extra', got %q", apps[0].Name)
	}
}

func TestParseGenericOutput_VeryLongLines(t *testing.T) {
	longName := strings.Repeat("a", 1000)
	longVersion := strings.Repeat("1", 500)
	input := fmt.Sprintf("%s,%s,Vendor", longName, longVersion)

	reader := strings.NewReader(input)
	apps, err := parseGenericOutput(reader, ",", "deb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(apps) != 1 {
		t.Fatalf("expected 1 app, got %d", len(apps))
	}
	if len(apps[0].Name) != 1000 {
		t.Errorf("name length mismatch")
	}
}

func TestParseGenericOutput_UnicodeCharacters(t *testing.T) {
	input := `日本語パッケージ,1.0,日本のベンダー
пакет,2.0,Русский вендор
émoji-pkg,3.0,Vendor
`
	reader := strings.NewReader(strings.TrimSpace(input))
	apps, err := parseGenericOutput(reader, ",", "deb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(apps) != 3 {
		t.Fatalf("expected 3 apps, got %d", len(apps))
	}
	if apps[0].Name != "日本語パッケージ" {
		t.Errorf("unicode name mismatch: %q", apps[0].Name)
	}
}

func TestFindApkVersionIndex_EdgeCases(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"a-1", 1},                   // Minimal valid
		{"-1.0", -1},                 // Starts with hyphen
		{"package-", -1},             // Ends with hyphen, no version
		{"package--1.0", 8},          // Double hyphen
		{"my-pkg-name-1.2.3-r4", 11}, // Version with revision
		{"", -1},                     // Empty string
		{"nodigits-abc", -1},         // No digit after hyphen
		{"pkg-1a2b3c", 3},            // Digit immediately after hyphen
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := findApkVersionIndex(tt.input)
			if got != tt.want {
				t.Errorf("findApkVersionIndex(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

// generatePkgOutput creates realistic FreeBSD pkg query output for benchmarks.
func generatePkgOutput(count int) string {
	packages := []struct {
		name       string
		version    string
		maintainer string
	}{
		{"bash", "5.2.37", "ports@FreeBSD.org"},
		{"curl", "8.11.1", "jhixson@FreeBSD.org"},
		{"git", "2.47.1", "lwhsu@FreeBSD.org"},
		{"go", "1.23.4", "jlaffaye@FreeBSD.org"},
		{"nginx", "1.26.2", "vanilla@FreeBSD.org"},
		{"node20", "20.18.1", "bhughes@FreeBSD.org"},
		{"openssh-portable", "9.9.p1", "des@FreeBSD.org"},
		{"pkg", "1.21.3", "pkg@FreeBSD.org"},
		{"python311", "3.11.11", "antoine@FreeBSD.org"},
		{"py311-pip", "24.3.1", "antoine@FreeBSD.org"},
		{"rust", "1.83.0", "rust@FreeBSD.org"},
		{"sqlite3", "3.47.2", "pavelivolkov@FreeBSD.org"},
		{"sudo", "1.9.16p2", "jem@FreeBSD.org"},
		{"vim", "9.1.0889", "adamw@FreeBSD.org"},
		{"zsh", "5.9_4", "bapt@FreeBSD.org"},
	}

	var b strings.Builder
	for i := range count {
		p := packages[i%len(packages)]
		fmt.Fprintf(&b, "%s,%s,%s\n", p.name, p.version, p.maintainer)
	}
	return b.String()
}

// generateDpkgOutput creates realistic dpkg output for benchmarks.
func generateDpkgOutput(count int) string {
	packages := []struct {
		name       string
		version    string
		maintainer string
	}{
		{"adduser", "3.118ubuntu5", "Ubuntu Core Developers <ubuntu-devel-discuss@lists.ubuntu.com>"},
		{"apt", "2.0.9", "APT Development Team <deity@lists.debian.org>"},
		{"base-files", "11ubuntu5.6", "Santiago Vila <sanvila@debian.org>"},
		{"bash", "5.0-6ubuntu1.2", "Matthias Klose <doko@debian.org>"},
		{"coreutils", "8.30-3ubuntu2", "Michael Stone <mstone@debian.org>"},
		{"curl", "7.68.0-1ubuntu2.18", "Alessandro Ghedini <ghedo@debian.org>"},
		{"dpkg", "1.19.7ubuntu3.2", "Dpkg Developers <debian-dpkg@lists.debian.org>"},
		{"gcc-10", "10.3.0-1ubuntu1~20.04", "Debian GCC Maintainers <debian-gcc@lists.debian.org>"},
		{"git", "1:2.25.1-1ubuntu3.11", "Gerrit Pape <pape@smarden.org>"},
		{"libc6", "2.31-0ubuntu9.12", "GNU Libc Maintainers <debian-glibc@lists.debian.org>"},
	}

	var b strings.Builder
	for i := range count {
		p := packages[i%len(packages)]
		fmt.Fprintf(&b, "%s,%s,%s\n", p.name, p.version, p.maintainer)
	}
	return b.String()
}

// generateApkOutput creates realistic Alpine apk output for benchmarks.
func generateApkOutput(count int) string {
	packages := []string{
		"musl-1.2.4-r2",
		"busybox-1.36.1-r5",
		"alpine-baselayout-3.4.3-r1",
		"openssl-3.1.4-r2",
		"zlib-1.3-r2",
		"apk-tools-2.14.0-r2",
		"libcrypto3-3.1.4-r2",
		"libssl3-3.1.4-r2",
		"ca-certificates-20230506-r0",
		"curl-8.4.0-r0",
	}

	var b strings.Builder
	for i := range count {
		fmt.Fprintf(&b, "%s\n", packages[i%len(packages)])
	}
	return b.String()
}

func benchmarkParse(b *testing.B, input, separator, vendor string) {
	b.Helper()
	for b.Loop() {
		reader := strings.NewReader(input)
		if _, err := parseGenericOutput(reader, separator, vendor); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseGenericOutput_Dpkg_100(b *testing.B) {
	benchmarkParse(b, generateDpkgOutput(100), ",", "deb")
}

func BenchmarkParseGenericOutput_Dpkg_500(b *testing.B) {
	benchmarkParse(b, generateDpkgOutput(500), ",", "deb")
}

func BenchmarkParseGenericOutput_Dpkg_1000(b *testing.B) {
	benchmarkParse(b, generateDpkgOutput(1000), ",", "deb")
}

func BenchmarkParseGenericOutput_Pkg_100(b *testing.B) {
	benchmarkParse(b, generatePkgOutput(100), ",", "freebsd")
}

func BenchmarkParseGenericOutput_Pkg_500(b *testing.B) {
	benchmarkParse(b, generatePkgOutput(500), ",", "freebsd")
}

func BenchmarkParseGenericOutput_Pkg_1000(b *testing.B) {
	benchmarkParse(b, generatePkgOutput(1000), ",", "freebsd")
}

func BenchmarkParseGenericOutput_Apk_100(b *testing.B) {
	benchmarkParse(b, generateApkOutput(100), "-", "alpine")
}

func BenchmarkParseGenericOutput_Apk_500(b *testing.B) {
	benchmarkParse(b, generateApkOutput(500), "-", "alpine")
}

func assertApp(t *testing.T, apps []protocol.Application, idx int, name, version, vendor string) {
	t.Helper()
	if idx >= len(apps) {
		t.Fatalf("app index %d out of range (len=%d)", idx, len(apps))
	}
	if apps[idx].Name != name {
		t.Errorf("app[%d].Name = %q, want %q", idx, apps[idx].Name, name)
	}
	if apps[idx].Version != version {
		t.Errorf("app[%d].Version = %q, want %q", idx, apps[idx].Version, version)
	}
	if apps[idx].Vendor != vendor {
		t.Errorf("app[%d].Vendor = %q, want %q", idx, apps[idx].Vendor, vendor)
	}
}
