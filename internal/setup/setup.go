package setup

import (
	"bufio"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nhdewitt/spectra/internal/fileutil"
	"golang.org/x/term"
)

const DefaultConfigPath = "/etc/spectra/server.json"

// ServerConfig is the persistent server configuration.
type ServerConfig struct {
	DatabaseURL string `json:"database_url"`
	ListenPort  int    `json:"listen_port"`
	ExternalURL string `json:"external_url,omitempty"`
	TLSCert     string `json:"tls_cert,omitempty"`
	TLSKey      string `json:"tls_key,omitempty"`
	TLSCA       string `json:"tls_ca,omitempty"`
	// TrustedProxies lists reverse proxies (CIDR or bare address) permitted to
	// set X-Forwarded-For / X-Real-IP. Empty means the headers are ignored.
	TrustedProxies []string `json:"trusted_proxies,omitempty"`
}

// AdminCredentials holds the admin user info collected during setup.
// The password is bcrypt-hashed.
type AdminCredentials struct {
	Username string
	Password string
}

// DBConfig holds the database connection details.
type DBConfig struct {
	Host    string
	Port    string
	Name    string
	User    string
	Pass    string
	SSLMode string
}

// DSN returns a PostgreSQL connection string.
func (d *DBConfig) DSN() string {
	return buildDSN(d.Host, d.Port, d.Name, d.User, d.Pass, d.SSLMode)
}

// TLSSetupConfig holds TLS setup parameters.
// Cert/Key/CA are pre-generated paths (from interactive prompting).
// If empty, RunSetup generates them from SANs.
type TLSSetupConfig struct {
	SANs []string
	Cert string
	Key  string
	CA   string
}

// SetupConfig is the shared configuration for both interactive and unattended setup.
type SetupConfig struct {
	DBConfig         *DBConfig
	CreateDB         bool
	Admin            *AdminCredentials
	Port             int
	TLS              *TLSSetupConfig // nil if TLS disabled
	ExternalURL      string
	SkipPrereqs      bool
	SkipServiceStart bool
	Interactive      bool
}

// ConfigExists confirms that the config file is present.
func ConfigExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// LoadConfig reads the server configuration.
func LoadConfig(path string) (*ServerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg ServerConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	return &cfg, nil
}

// SaveConfig writes the server configuration with 0600 permissions.
func SaveConfig(cfg *ServerConfig, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	return fileutil.WriteSecure(path, data)
}

// TablesExist checks if the database has been migrated.
func TablesExist(ctx context.Context, pool *pgxpool.Pool) bool {
	var exists bool
	err := pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'users')").Scan(&exists)
	return err == nil && exists
}

// PromptDBConfig collects database connection info interactively.
// When local is true, host/port/SSL are defaulted and not prompted.
func PromptDBConfig(reader *bufio.Reader, local bool) *DBConfig {
	fmt.Println("=== Database Configuration ===")

	db := &DBConfig{
		Host:    "localhost",
		Port:    "5432",
		Name:    "spectra",
		User:    "spectra",
		SSLMode: "disable",
	}

	if !local {
		db.Host = prompt(reader, "PostgreSQL host", db.Host)
		db.Port = prompt(reader, "PostgreSQL port", db.Port)
	}

	for {
		db.Name = prompt(reader, "Database name", db.Name)
		if !validIdent(db.Name) {
			fmt.Println("  [x] Database name may only contain letters, numbers, and underscores.")
			continue
		}
		break
	}

	for {
		db.User = prompt(reader, "Database user", db.User)
		if !validIdent(db.User) {
			fmt.Println("  [x] Username may only contain letters, numbers, and underscores.")
			continue
		}
		break
	}

	pass, err := promptPasswordConfirm()
	if err != nil {
		fmt.Printf("\n  [!] Fatal: could not read password: %v\n", err)
		os.Exit(1)
	}
	fmt.Println()
	db.Pass = pass

	if !local {
		for {
			useSSL := prompt(reader, "Use SSL (yes/no)", "no")
			switch strings.ToLower(useSSL) {
			case "yes":
				db.SSLMode = "require"
			case "no":
			default:
				continue
			}
			break
		}
	}

	return db
}

// PromptAdmin collects admin username and password interactively.
func PromptAdmin(reader *bufio.Reader) *AdminCredentials {
	fmt.Println("=== Admin Account ===")

	username := promptRequired(reader, "Username")

	password, err := promptPasswordConfirm()
	if err != nil {
		fmt.Printf("\n  [!] Fatal: could not read password: %v\n", err)
		os.Exit(1)
	}
	fmt.Println()

	return &AdminCredentials{
		Username: username,
		Password: password,
	}
}

// PromptPort collects the listen port.
func PromptPort(reader *bufio.Reader) int {
	for {
		portStr := prompt(reader, "Listen port", "8080")
		var port int
		if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil || port < 1 || port > 65535 {
			fmt.Println("  [x] Invalid port (1-65535).")
			continue
		}
		if port < 1024 {
			fmt.Printf("  [!] Port %d requires root.\n", port)
		}
		return port
	}
}

// PromptYesNo asks a yes/no question with a default.
func PromptYesNo(reader *bufio.Reader, label string, defaultYes bool) bool {
	def := "no"
	if defaultYes {
		def = "yes"
	}
	for {
		answer := prompt(reader, label+" (yes/no)", def)
		switch strings.ToLower(answer) {
		case "yes":
			return true
		case "no":
			return false
		}
	}
}

// PromptTLS asks whether to enable TLS and generates certs if yes.
// Returns nil if TLS is not enabled.
func PromptTLS(reader *bufio.Reader) *TLSSetupConfig {
	fmt.Println("=== TLS Configuration ===")

	if !PromptYesNo(reader, "Enable TLS", true) {
		return nil
	}

	// Collect SANs. The primary address is a replaceable default rather than a
	// forced entry (it lands in a cert whose only regen path also replaces the
	// CA, so a wrong detection here is expensive to undo).
	cands := detectLANCandidates()
	printAddrCandidates(cands)

	detectedIP := "127.0.0.1"
	if len(cands) > 0 {
		detectedIP = cands[0].IP.String()
	}
	primary := prompt(reader, "Primary address", detectedIP)

	fmt.Println("  Enter additional hostnames or IPs (comma-separated, or blank for none):")
	extra := prompt(reader, "Additional SANs", "")

	var sans []string
	if primary != "" {
		sans = append(sans, primary)
	}
	if extra != "" {
		for s := range strings.SplitSeq(extra, ",") {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			if net.ParseIP(s) != nil {
				sans = append(sans, s)
				continue
			}
			// Basic hostname validation: no spaces, no special chars
			if strings.ContainsAny(s, " \t!@#$%^&*()+=[]{}|\\;:'\"<>?/") {
				fmt.Printf("  [!] Skipping invalid SAN: %s\n", s)
				continue
			}
			sans = append(sans, s)
		}
	}

	return &TLSSetupConfig{SANs: sans}
}

// PromptExternalURL collects the externally-reachable server URL.
func PromptExternalURL(reader *bufio.Reader, port int, tlsEnabled bool) string {
	fmt.Println("=== External URL ===")
	fmt.Println("How agents and browsers reach this server.")

	detected := detectExternalURL(port, tlsEnabled)
	return prompt(reader, "External URL", detected)
}

// buildDSN constructs a PostgreSQL connection string with escaped credentials.
func buildDSN(host, port, dbName, user, pass, sslMode string) string {
	u := &url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(user, pass),
		Host:     net.JoinHostPort(host, port),
		Path:     dbName,
		RawQuery: "sslmode=" + url.QueryEscape(sslMode),
	}

	return u.String()
}

// detectExternalURL builds a default URL from the best-ranked local address.
func detectExternalURL(port int, tlsEnabled bool) string {
	scheme := "http"
	if tlsEnabled {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s:%d", scheme, detectLANIP(), port)
}

// ifaceAddr is one address paired with the interface it was found on.
type ifaceAddr struct {
	Iface string
	IP    net.IP
}

// AddrCandidate is a ranked local address. Reason is empty for a normal choice.
type AddrCandidate struct {
	Iface  string
	IP     net.IP
	Reason string
	score  int
}

// cgnat is the 100.64.0.0/10 shared address space. Tailscale assigns out of it,
// as do some ISPs.
var cgnat = net.IPNet{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)}

// virtualIfacePrefixes name interfaces belonging to VPNs, container runtimes, and
// hypervisors. An address on one of these is usually not how the rest of the
// network reaches this host.
//
// "br-" is deliberately hyphenated. That form is Docker's user-defined bridges,
// whereas a bare "br0" or Proxmox's "vmbr0" is frequently the real uplink and must
// not be demoted.
var virtualIfacePrefixes = []string{
	"br-", "cni", "docker", "flannel", "kube", "lxcbr", "podman",
	"tailscale", "tap", "tun", "utun", "veth", "virbr", "wg", "zt",
}

func isVirtualIface(name string) bool {
	n := strings.ToLower(name)
	for _, p := range virtualIfacePrefixes {
		if strings.HasPrefix(n, p) {
			return true
		}
	}
	return false
}

// rankAddrs orders candidates best-first. Pure so that the ordering rules can be
// tested directly, detectLANCandidates supplies the real input.
//
// Nothing is discarded outright except addresses that cannot serve at all. A host
// whose only address is in the CGNAT range still gets that rather than a fallback
// to loopback, it's just ranked last.
func rankAddrs(addrs []ifaceAddr) []AddrCandidate {
	out := make([]AddrCandidate, 0, len(addrs))

	for _, a := range addrs {
		ip4 := a.IP.To4()
		if ip4 == nil || ip4.IsLoopback() || ip4.IsUnspecified() {
			continue
		}

		c := AddrCandidate{Iface: a.Iface, IP: ip4}
		virtual := isVirtualIface(a.Iface)

		switch {
		case cgnat.Contains(ip4):
			c.score, c.Reason = 1, "CGNAT/VPN range"
		case ip4.IsLinkLocalUnicast():
			c.score, c.Reason = 1, "link-local"
		case virtual && ip4.IsPrivate():
			c.score, c.Reason = 10, "virtual interface"
		case virtual:
			c.score, c.Reason = 5, "virtual interface"
		case ip4.IsPrivate():
			c.score = 30
		default:
			c.score = 20
		}

		out = append(out, c)
	}

	slices.SortStableFunc(out, func(a, b AddrCandidate) int {
		return cmp.Compare(b.score, a.score)
	})
	return out
}

// detectLANCandidates returns every usable local address, best-first.
func detectLANCandidates() []AddrCandidate {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}

	var addrs []ifaceAddr
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		ifAddrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range ifAddrs {
			if ipnet, ok := addr.(*net.IPNet); ok {
				addrs = append(addrs, ifaceAddr{Iface: iface.Name, IP: ipnet.IP})
			}
		}
	}
	return rankAddrs(addrs)
}

// detectLANIP returns the best-ranked local IPv4 address.
func detectLANIP() string {
	cands := detectLANCandidates()
	if len(cands) == 0 {
		return "127.0.0.1"
	}
	return cands[0].IP.String()
}

// printAddrCandidates shows what was found and why anything was passed
// over, so a wrong pick is visible at the prompt.
func printAddrCandidates(cands []AddrCandidate) {
	if len(cands) == 0 {
		fmt.Println("  No usable addresses detected. Falling back to 127.0.0.1")
		return
	}
	fmt.Println("  Detected addresses:")
	for i, c := range cands {
		note := "[selected]"
		if i > 0 {
			note = ""
		}
		if c.Reason != "" {
			note = strings.TrimSpace(note + " skipped: " + c.Reason)
			if i == 0 {
				note = "[selected] (" + c.Reason + ")"
			}
		}
		fmt.Printf("    %d) %-15s %-12s %s\n", i+1, c.IP, c.Iface, note)
	}
}

func prompt(reader *bufio.Reader, label, defaultVal string) string {
	if defaultVal != "" {
		fmt.Printf("%s [%s]: ", label, defaultVal)
	} else {
		fmt.Printf("%s: ", label)
	}

	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" {
		return defaultVal
	}
	return input
}

func promptRequired(reader *bufio.Reader, label string) string {
	for {
		fmt.Printf("%s: ", label)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input != "" {
			return input
		}
		fmt.Println("  [!] This field is required")
	}
}

func promptPassword(label string) (string, error) {
	fmt.Print(label)
	pw, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return "", err
	}
	return string(pw), nil
}

func promptPasswordConfirm() (string, error) {
	for {
		pass, err := promptPassword("Password: ")
		if err != nil {
			return "", err
		}
		fmt.Println()

		if len(pass) < 8 {
			fmt.Println("  [!] Password must be at least 8 characters.")
			continue
		}

		confirm, err := promptPassword("Confirm password: ")
		if err != nil {
			return "", err
		}
		fmt.Println()

		if pass != confirm {
			fmt.Println("  [x] Passwords do not match.")
			continue
		}

		return pass, nil
	}
}

// printPostSetupInstructions prints post-install guidance for interactive runs.
// The service is already running by this point.
func printPostSetupInstructions(externalURL string) {
	fmt.Println()
	fmt.Println("Setup complete. The Spectra server is now running.")
	fmt.Println()
	fmt.Println("Verify:")
	fmt.Println("    systemctl status spectra-server")
	fmt.Println("    journalctl -u spectra-server -f")
	fmt.Println()
	fmt.Printf("Dashboard: %s\n", externalURL)
	fmt.Println("Provision agents from the dashboard.")
}
