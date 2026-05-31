package setup

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"go.yaml.in/yaml/v3"
)

// SetupFile is the YAML structure for non-interactive setup.
type SetupFile struct {
	Database struct {
		Host     string `yaml:"host"`
		Port     string `yaml:"port"`
		Name     string `yaml:"name"`
		User     string `yaml:"user"`
		Password string `yaml:"password"`
		SSL      string `yaml:"ssl"`
		Create   bool   `yaml:"create"` // auto-create user and database
	} `yaml:"database"`
	Admin struct {
		Username string `yaml:"username"`
		Password string `yaml:"password"`
	} `yaml:"admin"`
	Server struct {
		Port        int    `yaml:"port"`
		Migrations  string `yaml:"migrations"`
		ExternalURL string `yaml:"external_url"`
	} `yaml:"server"`
	TLS struct {
		Enabled bool     `yaml:"enabled"`
		SANs    []string `yaml:"sans"`
	} `yaml:"tls"`
	SkipPrerequisites bool `yaml:"skip_prerequisites"`
}

var validSSLModes = map[string]bool{
	"disable":     true,
	"allow":       true,
	"prefer":      true,
	"require":     true,
	"verify-ca":   true,
	"verify-full": true,
}

// LoadSetupFile reads and validates a YAML setup file.
func LoadSetupFile(path string) (*SetupFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading setup file: %w", err)
	}

	var sf SetupFile
	if err := yaml.Unmarshal(data, &sf); err != nil {
		return nil, fmt.Errorf("parsing setup file: %w", err)
	}

	if sf.Database.Host == "" {
		sf.Database.Host = "localhost"
	}
	if sf.Database.Port == "" {
		sf.Database.Port = "5432"
	}
	if sf.Database.Name == "" {
		sf.Database.Name = "spectra"
	}
	if sf.Database.User == "" {
		sf.Database.User = "spectra"
	}
	if sf.Database.SSL == "" {
		sf.Database.SSL = "disable"
	}
	if sf.Server.Port == 0 {
		sf.Server.Port = 8080
	}
	if sf.Server.Migrations == "" {
		sf.Server.Migrations = DefaultMigrationsPath
	}

	if sf.Database.Password == "" {
		return nil, fmt.Errorf("database.password is required")
	}
	if sf.Admin.Username == "" {
		return nil, fmt.Errorf("admin.username is required")
	}
	if sf.Admin.Password == "" {
		return nil, fmt.Errorf("admin.password is required")
	}
	if len(sf.Admin.Password) < 8 {
		return nil, fmt.Errorf("admin.password must be at least 8 characters")
	}
	dbPort, err := strconv.Atoi(sf.Database.Port)
	if err != nil || dbPort < 1 || dbPort > 65535 {
		return nil, fmt.Errorf("database.port must be 1-65535")
	}
	if sf.Server.Port < 1 || sf.Server.Port > 65535 {
		return nil, fmt.Errorf("server.port must be 1-65535")
	}
	if !validIdent(sf.Database.Name) {
		return nil, fmt.Errorf("database.name contains invalid characters")
	}
	if !validIdent(sf.Database.User) {
		return nil, fmt.Errorf("database.user contains invalid characters")
	}
	if !validSSLModes[sf.Database.SSL] {
		return nil, fmt.Errorf("database.ssl must be one of: disable, allow, prefer, require, verify-ca, verify-full")
	}

	return &sf, nil
}

// RunNonInteractive performs the full setup from a SetupFile without prompts.
func RunNonInteractive(ctx context.Context, sf *SetupFile, configPath string) error {
	sc := &SetupConfig{
		DBConfig: &DBConfig{
			Host:    sf.Database.Host,
			Port:    sf.Database.Port,
			Name:    sf.Database.Name,
			User:    sf.Database.User,
			Pass:    sf.Database.Password,
			SSLMode: sf.Database.SSL,
		},
		CreateDB:      sf.Database.Create,
		MigrationsDir: sf.Server.Migrations,
		Admin: &AdminCredentials{
			Username: sf.Admin.Username,
			Password: sf.Admin.Password,
		},
		Port:        sf.Server.Port,
		ExternalURL: sf.Server.ExternalURL,
		SkipPrereqs: sf.SkipPrerequisites,
	}

	if sf.TLS.Enabled {
		sc.TLS = &TLSSetupConfig{
			SANs: sf.TLS.SANs,
		}
	}

	if sc.ExternalURL == "" {
		sc.ExternalURL = detectExternalURL(sc.Port, sc.TLS != nil)
	}

	return RunSetup(ctx, sc, configPath)
}
