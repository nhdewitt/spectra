package setup

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nhdewitt/spectra/internal/database"
	"golang.org/x/crypto/bcrypt"
)

// RunSetup executes the full setup flow from a SetupConfig.
// Used by both interactive and unattended paths.
func RunSetup(ctx context.Context, sc *SetupConfig, configPath string) error {
	if sc.DBConfig == nil {
		return fmt.Errorf("database configuration is required")
	}
	if sc.Admin == nil {
		return fmt.Errorf("admin credentials are required")
	}
	if sc.MigrationsDir == "" {
		return fmt.Errorf("migrations directory is required")
	}
	if sc.Port < 1 || sc.Port > 65535 {
		return fmt.Errorf("listen port must be 1-65535")
	}

	// Prerequisites
	if !sc.SkipPrereqs {
		fmt.Println("=== Prerequisites ===")
		if err := EnsurePrerequisites(); err != nil {
			return fmt.Errorf("prerequisites: %w", err)
		}
		fmt.Println()
	}

	if sc.CreateDB && sc.DBConfig.Host != "localhost" && sc.DBConfig.Host != "127.0.0.1" {
		return fmt.Errorf("database creation requires a local database host (got %s)", sc.DBConfig.Host)
	}

	// Create DB if requested
	if sc.CreateDB {
		fmt.Print("Creating database and user... ")
		if err := CreateDatabase(sc.DBConfig.Name, sc.DBConfig.User, sc.DBConfig.Pass); err != nil {
			fmt.Println("FAILED")
			return fmt.Errorf("create database: %w", err)
		}
		fmt.Println("OK")
	}

	// Connect
	dsn := sc.DBConfig.DSN()
	fmt.Print("Testing connection... ")
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		fmt.Println("FAILED")
		return fmt.Errorf("database connection failed: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		fmt.Println("FAILED")
		return fmt.Errorf("database ping failed: %w", err)
	}
	fmt.Println("OK")

	// Transaction for migrations + admin
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Migrations
	fmt.Print("Running migrations... ")
	applied, err := RunMigrationsTx(ctx, tx, sc.MigrationsDir)
	if err != nil {
		fmt.Println("FAILED")
		return fmt.Errorf("migration failed: %w", err)
	}
	fmt.Printf("OK (%d applied)\n", applied)

	// Admin user
	hash, err := bcrypt.GenerateFromPassword([]byte(sc.Admin.Password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hashing password: %w", err)
	}

	fmt.Print("Creating/updating superadmin user... ")
	queries := database.New(tx)
	if err := queries.UpsertSuperadmin(ctx, database.UpsertSuperadminParams{
		Username: sc.Admin.Username,
		Password: string(hash),
	}); err != nil {
		fmt.Println("FAILED")
		return fmt.Errorf("creating superadmin: %w", err)
	}
	fmt.Println("OK")

	// Commit DB before TLS/config
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}

	var tlsCert, tlsKey, tlsCA string
	if sc.TLS != nil {
		if sc.TLS.Cert != "" {
			tlsCert = sc.TLS.Cert
			tlsKey = sc.TLS.Key
			tlsCA = sc.TLS.CA
		} else if ConfigExists("/etc/spectra/tls/server.crt") && ConfigExists("/etc/spectra/tls/server.key") && ConfigExists("/etc/spectra/tls/ca.crt") {
			fmt.Println("TLS certificates already exist, skipping generation.")
			tlsCert = "/etc/spectra/tls/server.crt"
			tlsKey = "/etc/spectra/tls/server.key"
			tlsCA = "/etc/spectra/tls/ca.crt"
		} else {
			fmt.Print("Generating TLS certificates... ")
			sans := sc.TLS.SANs
			if len(sans) == 0 {
				sans = []string{detectLANIP()}
			}
			files, err := GenerateTLS(sans)
			if err != nil {
				fmt.Println("FAILED")
				return fmt.Errorf("TLS generation failed: %w", err)
			}
			fmt.Println("OK")
			tlsCert = files.SrvCert
			tlsKey = files.SrvKey
			tlsCA = files.CACert
		}
	}

	cfg := &ServerConfig{
		DatabaseURL: dsn,
		ListenPort:  sc.Port,
		ExternalURL: sc.ExternalURL,
		TLSCert:     tlsCert,
		TLSKey:      tlsKey,
		TLSCA:       tlsCA,
	}

	fmt.Print("Saving configuration... ")
	if err := SaveConfig(cfg, configPath); err != nil {
		fmt.Println("FAILED")
		return fmt.Errorf("saving config: %w", err)
	}
	fmt.Println("OK")

	fmt.Printf("\nConfiguration saved to %s\n", configPath)
	fmt.Printf("Dashboard: %s\n", sc.ExternalURL)

	return nil
}
