package setup

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nhdewitt/spectra/internal/database"
	"go.yaml.in/yaml/v3"
	"golang.org/x/crypto/bcrypt"
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
	} `yaml:"database"`
	Admin struct {
		Username string `yaml:"username"`
		Password string `yaml:"password"`
	} `yaml:"admin"`
	Server struct {
		Port       int    `yaml:"port"`
		Migrations string `yaml:"migrations"`
	} `yaml:"server"`
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
		sf.Database.User = "postgres"
	}
	if sf.Database.SSL == "" {
		sf.Database.SSL = "disable"
	}
	if sf.Server.Port == 0 {
		sf.Server.Port = 8080
	}
	if sf.Server.Migrations == "" {
		sf.Server.Migrations = "internal/database/migrations"
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
	if sf.Server.Port < 1 || sf.Server.Port > 65535 {
		return nil, fmt.Errorf("server.port must be 1-65535")
	}

	return &sf, nil
}

// RunNonInteractive performs the full setup from a SetupFile without prompts.
func RunNonInteractive(ctx context.Context, sf *SetupFile, configPath string) error {
	dbURL := buildDSN(
		sf.Database.Host,
		sf.Database.Port,
		sf.Database.Name,
		sf.Database.User,
		sf.Database.Password,
		sf.Database.SSL,
	)

	fmt.Print("Testing database connection... ")
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		fmt.Println("FAILED")
		return fmt.Errorf("database connection failed: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		fmt.Println("FAILED")
		pool.Close()
		return fmt.Errorf("database ping failed: %w", err)
	}
	fmt.Println("OK")
	defer pool.Close()

	var migrated bool
	err = pool.QueryRow(ctx,
		"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'users')").Scan(&migrated)
	if err != nil {
		migrated = false
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if !migrated {
		fmt.Print("Running migrations... ")
		applied, err := RunMigrationsTx(ctx, tx, sf.Server.Migrations)
		if err != nil {
			fmt.Println("FAILED")
			return fmt.Errorf("migration failed: %w", err)
		}
		fmt.Printf("OK (%d applied)\n", applied)
	} else {
		fmt.Println("Database already migrated, skipping.")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(sf.Admin.Password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hashing password: %w", err)
	}

	fmt.Print("Creating admin user... ")
	queries := database.New(tx)
	if err := queries.CreateUser(ctx, database.CreateUserParams{
		Username: sf.Admin.Username,
		Password: string(hash),
		Role:     "admin",
	}); err != nil {
		fmt.Println("FAILED")
		return fmt.Errorf("creating admin user: %w", err)
	}
	fmt.Println("OK")

	cfg := &ServerConfig{
		DatabaseURL: dbURL,
		ListenPort:  sf.Server.Port,
	}

	fmt.Print("Saving configuration... ")
	if err := SaveConfig(cfg, configPath); err != nil {
		fmt.Println("FAILED")
		return fmt.Errorf("saving config: %w", err)
	}
	fmt.Println("OK")

	if err := tx.Commit(ctx); err != nil {
		os.Remove(configPath)
		return fmt.Errorf("commit failed: %w", err)
	}

	fmt.Printf("\nConfiguration saved to %s\n", configPath)
	return nil
}
