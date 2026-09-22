package setup

import (
	"bufio"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveAndLoadConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.json")

	cfg := &ServerConfig{
		DatabaseURL: "postgres://user:pass@localhost:5432/spectra?sslmode=disable",
		ListenPort:  8080,
	}

	if err := SaveConfig(cfg, path); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if loaded.DatabaseURL != cfg.DatabaseURL {
		t.Errorf("DatabaseURL = %s, want %s", loaded.DatabaseURL, cfg.DatabaseURL)
	}
	if loaded.ListenPort != cfg.ListenPort {
		t.Errorf("ListenPort = %d, want %d", loaded.ListenPort, cfg.ListenPort)
	}
}

func TestSaveConfig_CreatesDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deep", "server.json")

	cfg := &ServerConfig{DatabaseURL: "test", ListenPort: 8080}
	if err := SaveConfig(cfg, path); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("config file should exist")
	}
}

func TestLoadConfig_NotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path/server.json")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadConfig_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.json")
	os.WriteFile(path, []byte("not json"), 0600)

	_, err := LoadConfig(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestConfigExists_True(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.json")
	os.WriteFile(path, []byte("{}"), 0600)

	if !ConfigExists(path) {
		t.Error("expected true for existing file")
	}
}

func TestConfigExists_False(t *testing.T) {
	if ConfigExists("/nonexistent/path/server.json") {
		t.Error("expected false for missing file")
	}
}

func TestBuildDSN(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		port     string
		dbName   string
		dbUser   string
		dbPass   string
		sslMode  string
		wantHost string
		wantSSL  string
	}{
		{
			name:    "basic",
			host:    "localhost",
			port:    "5432",
			dbName:  "spectra",
			dbUser:  "postgres",
			dbPass:  "secret",
			sslMode: "disable",
		},
		{
			name:    "special chars in password",
			host:    "localhost",
			port:    "5432",
			dbName:  "spectra",
			dbUser:  "postgres",
			dbPass:  "p@ss:w/rd&more=stuff",
			sslMode: "require",
		},
		{
			name:    "special chars in username",
			host:    "localhost",
			port:    "5432",
			dbName:  "spectra",
			dbUser:  "user@domain",
			dbPass:  "pass",
			sslMode: "disable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dsn := buildDSN(tt.host, tt.port, tt.dbName, tt.dbUser, tt.dbPass, tt.sslMode)
			if dsn == "" {
				t.Fatal("buildDSN returned empty string")
			}
			if !strings.Contains(dsn, tt.host) {
				t.Errorf("DSN missing host: %s", dsn)
			}
			if !strings.Contains(dsn, tt.sslMode) {
				t.Errorf("DSN missing sslmode: %s", dsn)
			}
			if !strings.HasPrefix(dsn, "postgres://") {
				t.Errorf("DSN missing scheme: %s", dsn)
			}
		})
	}
}

func TestBuildDSN_PasswordEscaping(t *testing.T) {
	dsn := buildDSN("localhost", "5432", "spectra", "postgres", "p@ss:word", "disable")

	// The @ and : in the password must be escaped so they don't break URL parsing
	if strings.Contains(dsn, "p@ss:word@") {
		t.Errorf("password not escaped in DSN: %s", dsn)
	}
}

func TestPrompt_Default(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("\n"))
	got := prompt(reader, "Test", "default")
	if got != "default" {
		t.Errorf("prompt = %s, want default", got)
	}
}

func TestPrompt_UserInput(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("custom\n"))
	got := prompt(reader, "Test", "default")
	if got != "custom" {
		t.Errorf("prompt = %s, want custom", got)
	}
}

func TestPrompt_TrimWhitespace(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("  spaced  \n"))
	got := prompt(reader, "Test", "")
	if got != "spaced" {
		t.Errorf("prompt = %q, want %q", got, "spaced")
	}
}

func TestPromptRequired_RetriesOnEmpty(t *testing.T) {
	// First line empty, second has value
	reader := bufio.NewReader(strings.NewReader("\n\nactual\n"))
	got := promptRequired(reader, "Test")
	if got != "actual" {
		t.Errorf("promptRequired = %s, want actual", got)
	}
}

func TestFilterAndSortMigrations(t *testing.T) {
	got := filterAndSortMigrations([]string{
		"003_third.up.sql",
		"001_first.up.sql",
		"002_second.up.sql",
		"001_first.down.sql", // should be excluded
		"readme.md",          // should be excluded
	})

	want := []string{"001_first.up.sql", "002_second.up.sql", "003_third.up.sql"}
	if len(got) != len(want) {
		t.Fatalf("found %d migrations, want %d", len(got), len(want))
	}
	for i, f := range got {
		if f != want[i] {
			t.Errorf("migration[%d] = %s, want %s", i, f, want[i])
		}
	}
}

func TestFilterAndSortMigrations_Empty(t *testing.T) {
	got := filterAndSortMigrations(nil)
	if len(got) != 0 {
		t.Errorf("found %d migrations, want 0", len(got))
	}
}

func TestFindMigrations(t *testing.T) {
	got, err := findMigrations()
	if err != nil {
		t.Fatalf("findMigrations: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected at least one embedded migration")
	}
	for i := 1; i < len(got); i++ {
		if got[i-1] >= got[i] {
			t.Errorf("migrations not sorted: %s >= %s", got[i-1], got[i])
		}
	}
	for _, f := range got {
		if !strings.HasSuffix(f, "up.sql") {
			t.Errorf("non-migration file returned: %s", f)
		}
	}
}

func TestPromptPort_Valid(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("9090\n"))
	got := PromptPort(reader)
	if got != 9090 {
		t.Errorf("PromptPort = %d, want 9090", got)
	}
}

func TestPromptPort_Default(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("\n"))
	got := PromptPort(reader)
	if got != 8080 {
		t.Errorf("PromptPort = %d, want 8080", got)
	}
}

func TestPromptPort_InvalidThenValid(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("abc\n99999\n4000\n"))
	got := PromptPort(reader)
	if got != 4000 {
		t.Errorf("PromptPort = %d, want 4000", got)
	}
}

func TestPromptPort_Zero(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("0\n8080\n"))
	got := PromptPort(reader)
	if got != 8080 {
		t.Errorf("PromptPort = %d, want 8080", got)
	}
}

func TestDBConfig_DSN(t *testing.T) {
	db := &DBConfig{
		Host:    "localhost",
		Port:    "5432",
		Name:    "spectra",
		User:    "spectra",
		Pass:    "secret",
		SSLMode: "disable",
	}

	dsn := db.DSN()
	if !strings.HasPrefix(dsn, "postgres://") {
		t.Errorf("DSN missing scheme: %s", dsn)
	}
	if !strings.Contains(dsn, "localhost:5432") {
		t.Errorf("DSN missing host:port: %s", dsn)
	}
	if !strings.Contains(dsn, "sslmode=disable") {
		t.Errorf("DSN missing sslmode: %s", dsn)
	}
}

func TestDBConfig_DSN_SpecialPassword(t *testing.T) {
	db := &DBConfig{
		Host:    "localhost",
		Port:    "5432",
		Name:    "spectra",
		User:    "spectra",
		Pass:    "p@ss:w/rd",
		SSLMode: "disable",
	}

	dsn := db.DSN()
	// Special chars should be escaped
	if strings.Contains(dsn, "p@ss:w/rd@") {
		t.Errorf("password not properly escaped: %s", dsn)
	}
}

func TestPromptYesNo(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		defaultYes bool
		want       bool
	}{
		{"yes", "yes\n", false, true},
		{"no", "no\n", true, false},
		{"default yes", "\n", true, true},
		{"default no", "\n", false, false},
		{"invalid then yes", "maybe\nyes\n", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bufio.NewReader(strings.NewReader(tt.input))
			got := PromptYesNo(reader, "Test", tt.defaultYes)
			if got != tt.want {
				t.Errorf("PromptYesNo = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetectExternalURL(t *testing.T) {
	url := detectExternalURL(8080, false)
	if !strings.HasPrefix(url, "http://") {
		t.Errorf("expected http:// prefix, got %s", url)
	}
	if !strings.HasSuffix(url, ":8080") {
		t.Errorf("expected :8080 suffix, got %s", url)
	}
}

func TestDetectExternalURL_TLS(t *testing.T) {
	url := detectExternalURL(8080, true)
	if !strings.HasPrefix(url, "https://") {
		t.Errorf("expected https:// prefix, got %s", url)
	}
}

func TestSaveAndLoadConfig_WithTLS(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/server.json"

	cfg := &ServerConfig{
		DatabaseURL: "postgres://user:pass@localhost:5432/spectra",
		ListenPort:  8080,
		ExternalURL: "https://203.0.113.10:8080",
		TLSCert:     "/etc/spectra/tls/server.crt",
		TLSKey:      "/etc/spectra/tls/server.key",
		TLSCA:       "/etc/spectra/tls/ca.crt",
	}

	if err := SaveConfig(cfg, path); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if loaded.TLSCert != cfg.TLSCert {
		t.Errorf("TLSCert = %s, want %s", loaded.TLSCert, cfg.TLSCert)
	}
	if loaded.TLSKey != cfg.TLSKey {
		t.Errorf("TLSKey = %s, want %s", loaded.TLSKey, cfg.TLSKey)
	}
	if loaded.TLSCA != cfg.TLSCA {
		t.Errorf("TLSCA = %s, want %s", loaded.TLSCA, cfg.TLSCA)
	}
}

// --- Address ranking ---
// detectLANIP used to return the first non-loopback IPv4 the kernel reported,
// in enumeration order. Adding Tailscale to a host puts a 100.x address in
// that list; Docker and libvirt put RFC1918 ones there. rankAddrs exists so
// the ordering rules can be checked without depending on the test host.

func ipOf(t *testing.T, s string) net.IP {
	t.Helper()
	ip := net.ParseIP(s)
	if ip == nil {
		t.Fatalf("bad test IP %q", s)
	}
	return ip
}

func TestRankAddrs_PrefersRealLANOverTailscale(t *testing.T) {
	// Tailscale first in enumeration order, which is what the old
	// first-match implementation would have returned.
	got := rankAddrs([]ifaceAddr{
		{Iface: "tailscale0", IP: ipOf(t, "100.87.12.40")},
		{Iface: "eth0", IP: ipOf(t, "10.10.107.5")},
	})

	if len(got) != 2 {
		t.Fatalf("got %d candidates, want 2", len(got))
	}
	if got[0].IP.String() != "10.10.107.5" {
		t.Errorf("winner = %s, want 10.10.107.5", got[0].IP)
	}
	if got[0].Reason != "" {
		t.Errorf("winner carries reason %q, want none", got[0].Reason)
	}
	if got[1].Reason == "" {
		t.Error("tailscale candidate should record why it was passed over")
	}
}

func TestRankAddrs_PrefersRealLANOverDocker(t *testing.T) {
	got := rankAddrs([]ifaceAddr{
		{Iface: "docker0", IP: ipOf(t, "172.17.0.1")},
		{Iface: "eth0", IP: ipOf(t, "192.168.1.20")},
	})

	if got[0].IP.String() != "192.168.1.20" {
		t.Errorf("winner = %s, want 192.168.1.20", got[0].IP)
	}
}

// vmbr0 is the uplink on a Proxmox host, not a virtual aside. Demoting
// everything starting with "br" would have taken the real address out.
func TestRankAddrs_KeepsProxmoxBridge(t *testing.T) {
	got := rankAddrs([]ifaceAddr{
		{Iface: "vmbr0", IP: ipOf(t, "10.10.107.5")},
		{Iface: "docker0", IP: ipOf(t, "172.17.0.1")},
	})

	if got[0].Iface != "vmbr0" {
		t.Errorf("winner iface = %s, want vmbr0", got[0].Iface)
	}
	if got[0].Reason != "" {
		t.Errorf("vmbr0 demoted with reason %q", got[0].Reason)
	}
}

// A Docker bridge named br-<id> is a real virtual interface and should be
// demoted, unlike vmbr0 above.
func TestRankAddrs_DemotesHyphenatedDockerBridge(t *testing.T) {
	got := rankAddrs([]ifaceAddr{
		{Iface: "br-9f2c1a4b7d3e", IP: ipOf(t, "172.18.0.1")},
		{Iface: "eth0", IP: ipOf(t, "10.10.107.5")},
	})

	if got[0].Iface != "eth0" {
		t.Errorf("winner iface = %s, want eth0", got[0].Iface)
	}
}

// Falling back to loopback would be worse than a CGNAT address that at least
// reaches the host.
func TestRankAddrs_KeepsCGNATWhenItIsAll(t *testing.T) {
	got := rankAddrs([]ifaceAddr{
		{Iface: "tailscale0", IP: ipOf(t, "100.87.12.40")},
	})

	if len(got) != 1 {
		t.Fatalf("got %d candidates, want 1", len(got))
	}
	if got[0].IP.String() != "100.87.12.40" {
		t.Errorf("winner = %s, want the CGNAT address", got[0].IP)
	}
	if got[0].Reason == "" {
		t.Error("CGNAT winner should still say why it is not ideal")
	}
}

func TestRankAddrs_DropsLoopbackAndIPv6(t *testing.T) {
	got := rankAddrs([]ifaceAddr{
		{Iface: "lo", IP: ipOf(t, "127.0.0.1")},
		{Iface: "eth0", IP: ipOf(t, "fe80::1")},
		{Iface: "eth0", IP: ipOf(t, "10.10.107.5")},
	})

	if len(got) != 1 {
		t.Fatalf("got %d candidates, want 1", len(got))
	}
	if got[0].IP.String() != "10.10.107.5" {
		t.Errorf("winner = %s, want 10.10.107.5", got[0].IP)
	}
}

func TestRankAddrs_DemotesLinkLocal(t *testing.T) {
	got := rankAddrs([]ifaceAddr{
		{Iface: "eth1", IP: ipOf(t, "169.254.7.9")},
		{Iface: "eth0", IP: ipOf(t, "203.0.113.10")},
	})

	if got[0].IP.String() != "203.0.113.10" {
		t.Errorf("winner = %s, want 203.0.113.10", got[0].IP)
	}
}

// Two equally-ranked addresses must not reorder between runs, or the cert and
// the external URL could disagree across a reinstall.
func TestRankAddrs_StableForEqualScores(t *testing.T) {
	in := []ifaceAddr{
		{Iface: "eth0", IP: ipOf(t, "10.10.107.5")},
		{Iface: "eth1", IP: ipOf(t, "10.10.108.5")},
	}

	first := rankAddrs(in)
	for range 5 {
		again := rankAddrs(in)
		if again[0].IP.String() != first[0].IP.String() {
			t.Fatalf("ordering changed: %s then %s", first[0].IP, again[0].IP)
		}
	}
	if first[0].Iface != "eth0" {
		t.Errorf("winner iface = %s, want eth0 (input order preserved)", first[0].Iface)
	}
}

func TestRankAddrs_Empty(t *testing.T) {
	if got := rankAddrs(nil); len(got) != 0 {
		t.Errorf("got %d candidates from nil input", len(got))
	}
}
