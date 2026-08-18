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

// postBaselineMigrations returns the migrations that must genuinely execute on
// an existing install: everything ordered after migrationTrackingBaseline.
// Derived from the embedded FS so adding a migration needs no test edit.
func postBaselineMigrations(t *testing.T) []string {
	t.Helper()

	files, err := findMigrations()
	if err != nil {
		t.Fatalf("findMigrations: %v", err)
	}

	var out []string
	for _, f := range files {
		if migrationVersion(f) > migrationTrackingBaseline {
			out = append(out, f)
		}
	}
	return out
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
// migrations added after the tracking system's introduction must
// actually execute their real DDL, not just get their version recorded
// via backfill. Before the fix, this test would fail on both assertions -
// post-baseline migrations would end up "applied" in schema_migrations,
// but their DDL would never have actually run against the database.
func TestRunMigrations_ExecutesPostCutoffMigrationsOnExistingInstall(t *testing.T) {
	q := newFakeQuerier()

	want := postBaselineMigrations(t)
	if len(want) == 0 {
		t.Fatal("no post-baseline migrations found; baseline constant may be stale")
	}

	applied, err := runMigrations(context.Background(), q, true /* existingInstall */)
	if err != nil {
		t.Fatalf("runMigrations: %v", err)
	}
	if applied != len(want) {
		t.Errorf("applied = %d, want %d", applied, len(want))
	}

	for _, f := range want {
		v := migrationVersion(f)
		if !q.appliedVersions[v] {
			t.Errorf("%s should be recorded as applied", v)
		}
		if !q.ranRealDDL(t, f) {
			t.Errorf("%s's DDL was never executed — only backfilled, reproducing the incident", v)
		}
	}

	// Negative control: the baseline itself must be backfilled, not re-executed.
	if q.ranRealDDL(t, migrationTrackingBaseline+".up.sql") {
		t.Errorf("%s's DDL should not have been re-executed", migrationTrackingBaseline)
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
