package setup

import (
	"context"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nhdewitt/spectra/internal/database"
)

const migrationsSubdir = "migrations"

// migrationTrackingBaseline marks the schema_migrations tracking system's
// start date, not a maintenance TODO. It's referenced exactly once, the
// first time an existing install (pre-dating this tracking) starts up under
// it. After that one backfill, schema_migrations is authoritative and this
// constant is never consulted again for that install.
const migrationTrackingBaseline = "017_smtp_config"

const createSchemaMigrationsSQL = `
CREATE TABLE IF NOT EXISTS schema_migrations (
	version		TEXT PRIMARY KEY,
	applied_at	TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`

// querier is satisfied by both *pgxpool.Pool and pgx.Tx, letting the
// migration logic run identically under either.
type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// RunMigrations applies pending .up.sql files embedded in the binary
// (database.MigrationsFS), tracking applied versions in schema_migrations
// so this is safe to call on every server start - already-applied files are
// skipped rather than re-executed, and new files added in a later build
// get picked up automatically.
//
// On a database that predates schema_migrations tracking (TablesExist
// reports true, but the tracker is empty), versions up through
// migrationTrackingBaseline are backfilled as already-applied instead of
// re-executed - they already ran via the old blind-replay behavior before
// this tracking existed. Anything newer always executes for real.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool) (int, error) {
	return runMigrations(ctx, pool, TablesExist(ctx, pool))
}

// RunMigrationsTx is the transactional counterpart, used during initial
// setup where migrations and superadmin creation commit together. It never
// backfills: cmd/setup already gates on TablesExist before RunSetup is
// invoked at all, so reaching here always means a fresh install.
func RunMigrationsTx(ctx context.Context, tx pgx.Tx) (int, error) {
	return runMigrations(ctx, tx, false)
}

func runMigrations(ctx context.Context, q querier, existingInstall bool) (int, error) {
	if _, err := q.Exec(ctx, createSchemaMigrationsSQL); err != nil {
		return 0, fmt.Errorf("creating schema_migrations: %w", err)
	}

	files, err := findMigrations()
	if err != nil {
		return 0, err
	}
	applied, err := appliedVersions(ctx, q)
	if err != nil {
		return 0, err
	}

	if len(applied) == 0 && existingInstall {
		if err := backfillVersions(ctx, q, files); err != nil {
			return 0, err
		}
		applied, err = appliedVersions(ctx, q)
		if err != nil {
			return 0, err
		}
	}

	count := 0
	for _, f := range files {
		version := migrationVersion(f)
		if applied[version] {
			continue
		}

		sql, err := fs.ReadFile(database.MigrationsFS, migrationsSubdir+"/"+f)
		if err != nil {
			return count, fmt.Errorf("reading %s: %w", f, err)
		}
		if _, err := q.Exec(ctx, string(sql)); err != nil {
			return count, fmt.Errorf("applying %s: %w", f, err)
		}
		if _, err := q.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1);`, version); err != nil {
			return count, fmt.Errorf("recording %s: %w", f, err)
		}
		count++
	}

	return count, nil
}

// appliedVersions returns the set of migration versions already recorded
// in schema_migrations.
func appliedVersions(ctx context.Context, q querier) (map[string]bool, error) {
	rows, err := q.Query(ctx, `SELECT version FROM schema_migrations;`)
	if err != nil {
		return nil, fmt.Errorf("querying schema_migrations: %w", err)
	}
	defer rows.Close()

	out := make(map[string]bool)
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("scanning schema_migrations: %w", err)
		}
		out[v] = true
	}
	return out, rows.Err()
}

// backfillVersions records every file's version up through
// migrationTrackingBaseline as already-applied without executing it - the
// old blind-replay behavior genuinely ran these before tracking existed.
// Anything newer is deliberately left un-backfilled: it may have been
// added to the codebase without an intervening restart under the old
// system (exactly what happened with 018/019 in production - added, but
// never actually run, before this tracking was introduced), so it needs
// to actually execute in runMigrations' normal loop rather than be
// assumed done. Used at most once per pre-existing installation, the
// first time it's migrated under tracking.
func backfillVersions(ctx context.Context, q querier, files []string) error {
	for _, f := range files {
		version := migrationVersion(f)
		if version > migrationTrackingBaseline {
			continue
		}
		if _, err := q.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1) ON CONFLICT DO NOTHING;`, version); err != nil {
			return fmt.Errorf("backfilling %s: %w", version, err)
		}
	}
	return nil
}

// migrationVersion strips the .up.sql suffix
func migrationVersion(filename string) string {
	return strings.TrimSuffix(filename, ".up.sql")
}

// findMigrations lists embedded .up.sql filenames, sorted so they apply in
// numeric/lexical order.
func findMigrations() ([]string, error) {
	entries, err := fs.ReadDir(database.MigrationsFS, migrationsSubdir)
	if err != nil {
		return nil, fmt.Errorf("reading embedded migrations: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}

	return filterAndSortMigrations(names), nil
}

// filterAndSortMigrations keeps only .up.sql files and returns them sorted
// so migrations apply in numeric/lexical order. Pure and testable against
// synthetic filenames, split out specifically so this logic doesn't
// require touching the embedded filesystem to verify.
func filterAndSortMigrations(names []string) []string {
	var files []string
	for _, n := range names {
		if strings.HasSuffix(n, "up.sql") {
			files = append(files, n)
		}
	}
	sort.Strings(files)
	return files
}
