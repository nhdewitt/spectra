//go:build !windows

package collector

import (
	"fmt"
	"strings"
	"testing"
)

func TestParseGenericOutput_Dpkg(t *testing.T) {
	input := `
curl,7.81.0-1ubuntu1.16,Ubuntu Developers <ubuntu-devel-discuss@lists.ubuntu.com>
docker.io,24.0.5-0ubuntu1,Ubuntu Developers <ubuntu-devel-discuss@lists.ubuntu.com>
vim,2:8.2.3995-1ubuntu2.15,
`
	reader := strings.NewReader(strings.TrimSpace(input))
	apps, err := parseGenericOutput(reader, ",", "deb")
	if err != nil {
		t.Fatalf("Parser error: %v", err)
	}
	if len(apps) != 3 {
		t.Fatalf("Expected 3 apps, got %d", len(apps))
	}

	if apps[0].Name != "curl" || apps[0].Version != "7.81.0-1ubuntu1.16" {
		t.Errorf("Mismatch on curl: %+v", apps[0])
	}
	if apps[0].Vendor != "Ubuntu Developers" {
		t.Errorf("Vendor cleaning failed. Got %q, want %q", apps[0].Vendor, "Ubuntu Developers")
	}

	if apps[2].Name != "vim" {
		t.Errorf("Mismatch on vim: %+v", apps[2])
	}
	if apps[2].Vendor != "deb" {
		t.Errorf("Expected fallback vendor 'deb' for empty maintainer, got %q", apps[2].Vendor)
	}
}

func TestParseGenericOutput_Rpm(t *testing.T) {
	input := `
kernel,5.14.0,Red Hat, Inc.
hpptd,2.4.57,Apache Software Foundation
zsh,5.8,
`
	reader := strings.NewReader(strings.TrimSpace(input))
	apps, err := parseGenericOutput(reader, ",", "rpm")
	if err != nil {
		t.Fatalf("Parser error: %v", err)
	}
	if len(apps) != 3 {
		t.Fatalf("Expected 3 apps, got %d", len(apps))
	}

	if apps[0].Name != "kernel" || apps[0].Vendor != "Red Hat, Inc." {
		t.Errorf("Mismatch on kernel: %+v", apps[0])
	}

	if apps[2].Name != "zsh" || apps[2].Vendor != "rpm" {
		t.Errorf("Mismatch on zsh fallback: %+v", apps[2])
	}
}

func TestParseGenericOutput_Pacman(t *testing.T) {
	input := `
bash 5.1.16-1
go 2:1.20.5-1
linux 6.3.9.arch1-1
`
	reader := strings.NewReader(strings.TrimSpace(input))
	apps, err := parseGenericOutput(reader, " ", "arch")
	if err != nil {
		t.Fatalf("Parser error: %v", err)
	}
	if len(apps) != 3 {
		t.Fatalf("Expected 3 apps, got %d", len(apps))
	}

	if apps[0].Name != "bash" || apps[0].Version != "5.1.16-1" {
		t.Errorf("Mismatch on bash: %+v", apps[0])
	}
	if apps[0].Vendor != "arch" {
		t.Errorf("Expected vendor 'arch', got %q", apps[0].Vendor)
	}
}

func TestParseGenericOutput_Apk(t *testing.T) {
	input := `
alpine-baselayout-3.2.0-r23
busybox-1.35.0-r29
libretls-static-3.7.0-r0
mylib-1.0
`
	reader := strings.NewReader(strings.TrimSpace(input))
	apps, err := parseGenericOutput(reader, "-", "alpine")
	if err != nil {
		t.Fatalf("Parser error: %v", err)
	}
	if len(apps) != 4 {
		t.Fatalf("Expected 4 apps, got %d", len(apps))
	}

	tests := []struct {
		idx  int
		name string
		ver  string
	}{
		{0, "alpine-baselayout", "3.2.0-r23"},
		{1, "busybox", "1.35.0-r29"},
		{2, "libretls-static", "3.7.0-r0"},
		{3, "mylib", "1.0"},
	}

	for _, tt := range tests {
		got := apps[tt.idx]
		if got.Name != tt.name || got.Version != tt.ver {
			t.Errorf("Index %d: Expected %s / %s, got %s / %s", tt.idx, tt.name, tt.ver, got.Name, got.Version)
		}
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
	reader := strings.NewReader("    \n\t\n    \n")
	apps, err := parseGenericOutput(reader, ",", "deb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(apps) != 0 {
		t.Errorf("expected 0 apps, got %d", len(apps))
	}
}

func TestParseGenericOutput_MalformedLines(t *testing.T) {
	input := `
valid-package,1.0,Vendor
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

func TestParseGenericOutput_SpecialCharactgersInNames(t *testing.T) {
	input := `
lib++extra,1.0,Vendor
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
	if len(apps[0].Name) != 1000 || len(apps[0].Version) != 500 {
		t.Errorf("name/version length mismatch, expected %d (name) / %d (version), got %d / %d", len(longName), len(longVersion), len(apps[0].Name), len(apps[0].Version))
	}
}

func TestParseGenericOutput_UnicodeCharacters(t *testing.T) {
	input := `
日本語パッケージ,1.0,日本のベンダー
пакет,2.0,Русский вендор
émoji-pkg,3.0,Ço̷̲̱͖̠̖͉̠̻ͅm̴̬͍̹̞̘͇̹̪͎̟̜͍̗pa̡̛͕̗͔̩̗͎̘̳̘̮̠͕͜ͅn͏̵̧̛̱y
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

func TestFindApkVersionIndex(t *testing.T) {
	tests := []struct {
		input string
		want  int // Index of the hyphen before the version
	}{
		{"package-1.2.3", 7},
		{"my-package-2.0", 10},
		{"simple-1", 6},
		{"noversion", -1},
		{"bad-format-v1", -1},
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

func TestFindApkVersionIndex_EdgeCases(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"a-1", 1},                  // Minimal valid
		{"-1.0", -1},                // Starts with hyphen
		{"package-", -1},            // Ends with hyphen, no version
		{"package--1.0", 8},         // Double hyphen
		{"my-pkg-name-1.23-r4", 11}, // Version with revision
		{"", -1},                    // Empty string
		{"nodigits-abc", -1},        // No digit after hyphen
		{"pkg-1a2b3c", 3},           // Digit immediately after hyphen
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

func TestCleanVendor(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Ubuntu Developers <ubuntu-devel@lists.ubuntu.com>", "Ubuntu Developers"},
		{"Simple Vendor", "Simple Vendor"},
		{"<only@email.com>", ""},
		{"", ""},
		{"  Spaced Vendor  <email@test.com>  ", "Spaced Vendor"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := cleanVendor(tt.input)
			if got != tt.want {
				t.Errorf("cleanVendor(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func generateDpkgOutput(n int) string {
	var sb strings.Builder
	vendors := []string{
		"Ubuntu Developers <ubuntu-devel-discuss@lists.ubuntu.com>",
		"Debian Developers <debian-devel@lists.debian.org>",
		"",
		"Canonical Ltd.",
		"Red Hat, Inc. <packages@redhat.com>",
	}

	for i := range n {
		name := fmt.Sprintf("package-%d", i)
		version := fmt.Sprintf("%d.%d.%d-ubuntu%d", i%10, i%20, i%5, i%3)
		vendor := vendors[i%len(vendors)]
		sb.WriteString(fmt.Sprintf("%s,%s,%s\n", name, version, vendor))
	}
	return sb.String()
}

func generateApkOutput(n int) string {
	var sb strings.Builder
	prefixes := []string{"lib", "py3-", "go-", "rust-", "alpine-", ""}

	for i := range n {
		prefix := prefixes[i%len(prefixes)]
		name := fmt.Sprintf("%spackage%d", prefix, i)
		version := fmt.Sprintf("%d.%d.%d-r%d", i%10, i%20, i%5, i%3)
		sb.WriteString(fmt.Sprintf("%s-%s\n", name, version))
	}
	return sb.String()
}

func BenchmarkParseGenericOutput_Dpkg_100(b *testing.B) {
	input := generateDpkgOutput(100)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		r := strings.NewReader(input)
		_, _ = parseGenericOutput(r, ",", "deb")
	}
}

func BenchmarkParseGenericOutput_Dpkg_500(b *testing.B) {
	input := generateDpkgOutput(500)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		r := strings.NewReader(input)
		_, _ = parseGenericOutput(r, ",", "deb")
	}
}

func BenchmarkParseGenericOutput_Dpkg_1000(b *testing.B) {
	input := generateDpkgOutput(1000)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		r := strings.NewReader(input)
		_, _ = parseGenericOutput(r, ",", "deb")
	}
}

func BenchmarkParseGenericOutput_Apk_100(b *testing.B) {
	input := generateApkOutput(100)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		r := strings.NewReader(input)
		_, _ = parseGenericOutput(r, "-", "alpine")
	}
}

func BenchmarkParseGenericOutput_Apk_500(b *testing.B) {
	input := generateApkOutput(500)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		r := strings.NewReader(input)
		_, _ = parseGenericOutput(r, "-", "alpine")
	}
}
