package setup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSetupFile_Defaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "setup.yaml")

	content := `
database:
  password: secretpass
admin:
  username: admin
  password: longpassword
`
	os.WriteFile(path, []byte(content), 0644)

	sf, err := LoadSetupFile(path)
	if err != nil {
		t.Fatalf("LoadSetupFile: %v", err)
	}

	if sf.Database.Host != "localhost" {
		t.Errorf("Host = %s, want localhost", sf.Database.Host)
	}
	if sf.Database.Port != "5432" {
		t.Errorf("Port = %s, want 5432", sf.Database.Port)
	}
	if sf.Database.Name != "spectra" {
		t.Errorf("Name = %s, want spectra", sf.Database.Name)
	}
	if sf.Database.User != "spectra" {
		t.Errorf("User = %s, want spectra", sf.Database.User)
	}
	if sf.Database.SSL != "disable" {
		t.Errorf("SSL = %s, want disable", sf.Database.SSL)
	}
	if sf.Server.Port != 8080 {
		t.Errorf("Port = %d, want 8080", sf.Server.Port)
	}
	if sf.Server.Migrations != DefaultMigrationsPath {
		t.Errorf("Migrations = %s, want %s", sf.Server.Migrations, DefaultMigrationsPath)
	}
}

func TestLoadSetupFile_Validation(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name: "missing db password",
			content: `
admin:
  username: admin
  password: longpassword
`,
			wantErr: "database.password is required",
		},
		{
			name: "missing admin username",
			content: `
database:
  password: dbpass
admin:
  password: longpassword
`,
			wantErr: "admin.username is required",
		},
		{
			name: "missing admin password",
			content: `
database:
  password: dbpass
admin:
  username: admin
`,
			wantErr: "admin.password is required",
		},
		{
			name: "short admin password",
			content: `
database:
  password: dbpass
admin:
  username: admin
  password: short
`,
			wantErr: "admin.password must be at least 8 characters",
		},
		{
			name: "invalid port",
			content: `
database:
  password: dbpass
admin:
  username: admin
  password: longpassword
server:
  port: 99999
`,
			wantErr: "server.port must be 1-65535",
		},
		{
			name: "non-numeric db port",
			content: `
database:
  password: dbpass
  port: abc
admin:
  username: admin
  password: longpassword
`,
			wantErr: "database.port must be 1-65535",
		},
		{
			name: "invalid db name",
			content: `
database:
  password: dbpass
  name: "drop;table"
admin:
  username: admin
  password: longpassword
`,
			wantErr: "database.name contains invalid characters",
		},
		{
			name: "invalid db user",
			content: `
database:
  password: dbpass
  user: "user@host"
admin:
  username: admin
  password: longpassword
`,
			wantErr: "database.user contains invalid characters",
		},
		{
			name: "invalid ssl mode",
			content: `
database:
  password: dbpass
  ssl: bogus
admin:
  username: admin
  password: longpassword
`,
			wantErr: "database.ssl must be one of",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "setup.yaml")
			os.WriteFile(path, []byte(tt.content), 0644)

			_, err := LoadSetupFile(path)
			if err == nil {
				t.Fatal("expected error")
			}
			if !contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestLoadSetupFile_FullConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "setup.yaml")

	content := `
database:
  host: db.example.com
  port: "5433"
  name: mydb
  user: myuser
  password: mypassword
  ssl: require
  create: true
admin:
  username: superadmin
  password: adminpass123
server:
  port: 9090
  migrations: /opt/spectra/migrations
  external_url: https://spectra.example.com
tls:
  enabled: true
  sans:
    - 10.0.0.1
    - spectra.local
skip_prerequisites: true
`
	os.WriteFile(path, []byte(content), 0644)

	sf, err := LoadSetupFile(path)
	if err != nil {
		t.Fatalf("LoadSetupFile: %v", err)
	}

	if sf.Database.Host != "db.example.com" {
		t.Errorf("Host = %s", sf.Database.Host)
	}
	if sf.Database.Port != "5433" {
		t.Errorf("Port = %s", sf.Database.Port)
	}
	if sf.Database.SSL != "require" {
		t.Errorf("SSL = %s", sf.Database.SSL)
	}
	if !sf.Database.Create {
		t.Error("Create should be true")
	}
	if sf.Server.Port != 9090 {
		t.Errorf("Port = %d", sf.Server.Port)
	}
	if !sf.TLS.Enabled {
		t.Error("TLS should be enabled")
	}
	if len(sf.TLS.SANs) != 2 {
		t.Errorf("SANs = %v", sf.TLS.SANs)
	}
	if !sf.SkipPrerequisites {
		t.Error("SkipPrerequisites should be true")
	}
}

func TestLoadSetupFile_NotFound(t *testing.T) {
	_, err := LoadSetupFile("/nonexistent/setup.yaml")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadSetupFile_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "setup.yaml")
	os.WriteFile(path, []byte("{{invalid yaml"), 0644)

	_, err := LoadSetupFile(path)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
