package setup

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/nhdewitt/spectra/internal/database"
)

type fakeRows struct {
	versions []string
	pos      int
}

func (r *fakeRows) Close()                                       {}
func (r *fakeRows) Err() error                                   { return nil }
func (r *fakeRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *fakeRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *fakeRows) Values() ([]any, error)                       { return nil, nil }
func (r *fakeRows) RawValues() [][]byte                          { return nil }
func (r *fakeRows) Conn() *pgx.Conn                              { return nil }

func (r *fakeRows) Next() bool {
	if r.pos >= len(r.versions) {
		return false
	}
	r.pos++
	return true
}

func (r *fakeRows) Scan(dest ...any) error {
	if len(dest) != 1 {
		return fmt.Errorf("fakeRows.Scan: expected 1 dest, got %d", len(dest))
	}
	ptr, ok := dest[0].(*string)
	if !ok {
		return fmt.Errorf("fakeRows.Scan: expected *string dest")
	}
	*ptr = r.versions[r.pos-1]
	return nil
}

type fakeQuerier struct {
	appliedVersions map[string]bool
	execCalls       []string
}

func newFakeQuerier() *fakeQuerier {
	return &fakeQuerier{appliedVersions: make(map[string]bool)}
}

func (f *fakeQuerier) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	f.execCalls = append(f.execCalls, sql)
	if strings.Contains(sql, "INSERT INTO schema_migrations") && len(args) == 1 {
		if v, ok := args[0].(string); ok {
			f.appliedVersions[v] = true
		}
	}
	return pgconn.CommandTag{}, nil
}

func (f *fakeQuerier) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	versions := make([]string, 0, len(f.appliedVersions))
	for v := range f.appliedVersions {
		versions = append(versions, v)
	}
	return &fakeRows{versions: versions}, nil
}

func (f *fakeQuerier) ranRealDDL(t *testing.T, filename string) bool {
	t.Helper()
	content, err := readMigrationContent(filename)
	if err != nil {
		t.Fatalf("reading embedded %s: %v", filename, err)
	}
	for _, call := range f.execCalls {
		if call == content {
			return true
		}
	}
	return false
}

func readMigrationContent(filename string) (string, error) {
	b, err := database.MigrationsFS.ReadFile(migrationsSubdir + "/" + filename)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// TestBackfillVersions_CutoffBoundary tests the exact boundary with
// synthetic filenames, independent of whatever the real migration set
// happens to contain at any given time.
func TestBackfillVersions_CutoffBoundary(t *testing.T) {
	files := []string{
		"016_x.up.sql",
		"017_smtp_config.up.sql", // == cutoff, should backfill
		"018_y.up.sql",           // > cutoff, should NOT backfill
	}
	q := newFakeQuerier()

	if err := backfillVersions(context.Background(), q, files); err != nil {
		t.Fatalf("backfillVersions: %v", err)
	}

	if !q.appliedVersions["016_x"] {
		t.Error("016_x should have been backfilled (below cutoff)")
	}
	if !q.appliedVersions["017_smtp_config"] {
		t.Error("017_smtp_config should have been backfilled (at cutoff)")
	}
	if q.appliedVersions["018_y"] {
		t.Error("018_y should NOT have been backfilled (above cutoff)")
	}
}

// TestBackfillVersions_OnlyBackfillsPreTrackingVersions runs against the
// real embedded migration set (not synthetic names), confirming exactly
// which of the actual files in the repo right now get backfilled. This
// will need updating if lastPreTrackingMigration is ever intentionally
// changed, which is the point - it pins current behavior against drift.
func TestBackfillVersions_OnlyBackfillsPreTrackingVersions(t *testing.T) {
	files, err := findMigrations()
	if err != nil {
		t.Fatalf("findMigrations: %v", err)
	}
	q := newFakeQuerier()

	if err := backfillVersions(context.Background(), q, files); err != nil {
		t.Fatalf("backfillVersions: %v", err)
	}

	if !q.appliedVersions["017_smtp_config"] {
		t.Error("017_smtp_config should have been backfilled")
	}
	if q.appliedVersions["018_status_thresholds"] {
		t.Error("018_status_thresholds should NOT have been backfilled - this is exactly the bug that broke production")
	}
	if q.appliedVersions["019_overview_indexes"] {
		t.Error("019_overview_indexes should NOT have been backfilled")
	}
}

// TestRunMigrations_ExecutesPostCutoffMigrationsOnExistingInstall is the
// direct regression test for the production incident: on an existing
// install (empty schema_migrations, TablesExist would report true), the
// two migrations added after the tracking system's introduction must
// actually execute their real DDL, not just get their version recorded
// via backfill. Before the fix, this test would fail on both assertions -
// 018/019 would end up "applied" in schema_migrations, but their DDL
// would never have actually run against the database.
func TestRunMigrations_ExecutesPostCutoffMigrationsOnExistingInstall(t *testing.T) {
	q := newFakeQuerier()

	applied, err := runMigrations(context.Background(), q, true /* existingInstall */)
	if err != nil {
		t.Fatalf("runMigrations: %v", err)
	}

	// Exactly the two post-cutoff migrations should have actually executed.
	if applied != 2 {
		t.Errorf("applied = %d, want 2 (018 and 019 executing for real)", applied)
	}

	if !q.appliedVersions["018_status_thresholds"] {
		t.Error("018_status_thresholds should be recorded as applied after running")
	}
	if !q.appliedVersions["019_overview_indexes"] {
		t.Error("019_overview_indexes should be recorded as applied after running")
	}

	// The critical distinction: was the real DDL executed, or only the
	// bookkeeping row inserted? This is what actually would have caught
	// the incident - schema_migrations claimed 018 was applied, but
	// CREATE TABLE status_thresholds had never run.
	if !q.ranRealDDL(t, "018_status_thresholds.up.sql") {
		t.Error("018_status_thresholds's actual DDL was never executed - only backfilled, reproducing the incident")
	}
	if !q.ranRealDDL(t, "019_overview_indexes.up.sql") {
		t.Error("019_overview_indexes's actual DDL was never executed - only backfilled")
	}

	// Sanity check the other direction: an older, genuinely-pre-tracking
	// migration should NOT have had its DDL re-executed (only backfilled).
	if q.ranRealDDL(t, "017_smtp_config.up.sql") {
		t.Error("017_smtp_config's DDL should not have been re-executed - it should only be backfilled")
	}
}

// TestRunMigrations_FreshInstallExecutesEverything confirms the
// existingInstall=false path (RunMigrationsTx's contract - "never
// backfills") runs every migration's real DDL, matching a genuinely fresh
// database with no tables at all yet.
func TestRunMigrations_FreshInstallExecutesEverything(t *testing.T) {
	q := newFakeQuerier()
	files, err := findMigrations()
	if err != nil {
		t.Fatalf("findMigrations: %v", err)
	}

	applied, err := runMigrations(context.Background(), q, false /* existingInstall */)
	if err != nil {
		t.Fatalf("runMigrations: %v", err)
	}

	if applied != len(files) {
		t.Errorf("applied = %d, want %d (every migration, fresh install)", applied, len(files))
	}
	if !q.ranRealDDL(t, "001_core.up.sql") {
		t.Error("001_core's DDL should have executed on a fresh install, not been backfilled")
	}
}
