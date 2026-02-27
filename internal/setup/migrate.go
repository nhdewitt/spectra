package setup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RunMigrations applies all .up.sql files using a connection pool.
// Each migration runs independently (use RunMigrationsTx for atomic execution).
func RunMigrations(ctx context.Context, pool *pgxpool.Pool, migrationsDir string) (int, error) {
	files, err := findMigrations(migrationsDir)
	if err != nil {
		return 0, err
	}

	applied := 0
	for _, f := range files {
		sql, err := os.ReadFile(filepath.Join(migrationsDir, f))
		if err != nil {
			return applied, fmt.Errorf("reading %s: %w", f, err)
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			return applied, fmt.Errorf("applying %s: %w", f, err)
		}
		applied++
	}

	return applied, nil
}

func RunMigrationsTx(ctx context.Context, tx pgx.Tx, migrationsDir string) (int, error) {
	files, err := findMigrations(migrationsDir)
	if err != nil {
		return 0, err
	}

	applied := 0
	for _, f := range files {
		sql, err := os.ReadFile(filepath.Join(migrationsDir, f))
		if err != nil {
			return applied, fmt.Errorf("reading %s: %w", f, err)
		}
		if _, err := tx.Exec(ctx, string(sql)); err != nil {
			return applied, fmt.Errorf("applying %s: %w", f, err)
		}
		applied++
	}

	return applied, nil
}

func findMigrations(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading migrations dir: %w", err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), "up.sql") {
			files = append(files, e.Name())
		}
	}

	sort.Strings(files)
	return files, nil
}
