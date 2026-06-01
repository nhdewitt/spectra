package setup

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nhdewitt/spectra/internal/fileutil"
	"golang.org/x/term"
)

const (
	DefaultConfigPath     = "/etc/spectra/server.json"
	DefaultMigrationsPath = "internal/database/migrations"
)

// ServerConfig is the persistent server configuration.
type ServerConfig struct {
	DatabaseURL string `json:"database_url"`
	ListenPort  int    `json:"listen_port"`
	ExternalURL string `json:"external_url,omitempty"`
	TLSCert     string `json:"tls_cert,omitempty"`
	TLSKey      string `json:"tls_key,omitempty"`
	TLSCA       string `json:"tls_ca,omitempty"`
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
	DBConfig      *DBConfig
	CreateDB      bool
	MigrationsDir string
	Admin         *AdminCredentials
	Port          int
	TLS           *TLSSetupConfig // nil if TLS disabled
	ExternalURL   string
	SkipPrereqs   bool
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

// PromptMigrationsDir collects the directory containing the DB migrations.
func PromptMigrationsDir(reader *bufio.Reader) string {
	var migDir string
	for {
		migDir = prompt(reader, "Migrations directory", "internal/database/migrations")
		if _, err := os.Stat(migDir); os.IsNotExist(err) {
			fmt.Printf("  [x] Directory not found: %s", migDir)
			continue
		}
		return migDir
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

	// Collect additional SANs
	detectedIP := detectLANIP()
	fmt.Printf("  Detected LAN IP: %s\n", detectedIP)
	fmt.Println("  Enter additional hostnames or IPs (comma-separated, or blank for none):")
	extra := prompt(reader, "Additional SANs", "")

	sans := []string{detectedIP}
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

// detectExternalURL finds the first non-loopback IPv4 address and
// builds a default URL from it.
func detectExternalURL(port int, tlsEnabled bool) string {
	scheme := "http"
	if tlsEnabled {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s:%d", scheme, detectLANIP(), port)
}

// detectLANIP returns the first non-loopback IPv4 address.
func detectLANIP() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "127.0.0.1"
	}

	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "127.0.0.1"
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
