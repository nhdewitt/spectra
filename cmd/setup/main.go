package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nhdewitt/spectra/internal/database"
	"github.com/nhdewitt/spectra/internal/setup"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	configPath := flag.String("config", setup.DefaultConfigPath, "Path to save server config")
	flag.Parse()

	ctx := context.Background()
	reader := bufio.NewReader(os.Stdin)

	fmt.Println()
	fmt.Println("Spectra Server Setup")
	fmt.Println("====================")
	fmt.Println()

	// Database
	dbURL := setup.PromptDB(reader)
	fmt.Println()

	fmt.Print("Testing connection... ")
	pool, err := database.NewPool(ctx, dbURL)
	if err != nil {
		fmt.Println("FAILED")
		log.Fatalf("Could not connect to database: %v", err)
	}
	fmt.Println("OK")
	fmt.Println()

	migrated := tablesExist(ctx, pool)
	var migrationsDir string
	if !migrated {
		migrationsDir = setup.PromptMigrationsDir(reader)
		fmt.Println()
	}

	// Collect remaining input
	admin := setup.PromptAdmin(reader)

	hash, err := bcrypt.GenerateFromPassword([]byte(admin.PasswordHash), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}

	port := setup.PromptPort(reader)
	fmt.Println()

	cfg := &setup.ServerConfig{
		DatabaseURL: dbURL,
		ListenPort:  port,
	}

	// Start a transaction, allow for rollback on failure.
	tx, err := pool.Begin(ctx)
	if err != nil {
		log.Fatalf("Failed to begin transaction: %v", err)
	}
	defer tx.Rollback(ctx)

	// Run migrations if DB tables don't already exist.
	if !migrated {
		fmt.Print("Running migration... ")
		applied, err := setup.RunMigrationsTx(ctx, tx, migrationsDir)
		if err != nil {
			fmt.Println("FAILED")
			tx.Rollback(ctx)
			log.Fatalf("Migration failed: %v", err)
		}
		fmt.Printf("OK (%d applied)\n", applied)
	}

	// Create admin
	fmt.Print("Creating admin user... ")
	queries := database.New(tx)
	if err := queries.CreateUser(ctx, database.CreateUserParams{
		Username: admin.Username,
		Password: string(hash),
		Role:     "admin",
	}); err != nil {
		fmt.Println("FAILED")
		tx.Rollback(ctx)
		log.Fatalf("Failed to create admin user: %v", err)
	}
	fmt.Println("OK")

	// Save config before committing, rollback on failure
	fmt.Print("Saving configuration... ")
	if err := setup.SaveConfig(cfg, *configPath); err != nil {
		fmt.Println("FAILED")
		if errors.Is(err, os.ErrPermission) {
			fmt.Printf("  [!] Permission denied. Try: sudo %s\n", os.Args[0])
		}
		tx.Rollback(ctx)
		log.Fatalf("[!] Fatal: %v", err)
	}
	fmt.Println("OK")

	// Commit
	if err := tx.Commit(ctx); err != nil {
		// Remove config on commit error
		os.Remove(*configPath)
		log.Fatalf("Failed to commit database changes: %v", err)
	}

	pool.Close()

	fmt.Println()
	fmt.Printf("Configuration saved to %s\n", *configPath)
	fmt.Println("Run spectra-server to start.")
}

func tablesExist(ctx context.Context, pool *pgxpool.Pool) (exists bool) {
	err := pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'users')").Scan(&exists)
	return err == nil && exists
}
