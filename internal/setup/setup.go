package setup

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/nhdewitt/spectra/internal/fileutil"
	"golang.org/x/term"
)

const DefaultConfigPath = "/etc/spectra/server.json"

// ServerConfig is the persistent server configuration.
type ServerConfig struct {
	DatabaseURL string `json:"database_url"`
	ListenPort  int    `json:"listen_port"`
}

// AdminCredentials holds the admin user info collected during setup.
// The password is bcrypt-hashed.
type AdminCredentials struct {
	Username     string
	PasswordHash string
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

// PromptDB collects database connection info interactively and
// returns a Postgres connection string.
func PromptDB(reader *bufio.Reader) string {
	fmt.Println("=== Spectra Database Configuration ===")

	host := prompt(reader, "PostgreSQL host", "localhost")
	port := prompt(reader, "PostgreSQL port", "5432")

	var dbName string
	for {
		dbName = prompt(reader, "Database name", "spectra")
		if len(dbName) > 63 {
			fmt.Println("  [x] Database name too long (max 63 characters).")
			continue
		}
		break
	}

	var dbUser string
	for {
		dbUser = prompt(reader, "Username", "postgres")
		if len(dbUser) > 63 {
			fmt.Println("  [x] Username too long (max 63 characters).")
			continue
		}
		break
	}

	dbPass, err := promptPassword("Password: ")
	if err != nil {
		fmt.Printf("\n  [!] Fatal: coult not read password: %v\n", err)
		os.Exit(1)
	}
	fmt.Println()

	sslMode := "disable"
	for {
		useSSL := prompt(reader, "Use SSL (yes/no)", "no")
		switch strings.ToLower(useSSL) {
		case "yes":
			sslMode = "require"
		case "no":
		default:
			continue
		}
		break
	}

	return buildDSN(host, port, dbName, dbUser, dbPass, sslMode)
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

// PromptAdmin collects admin username and password interactively.
// Returns credentials with the password bcrypt-hashed.
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
		Username:     username,
		PasswordHash: password,
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
		break
	}
	return migDir
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
