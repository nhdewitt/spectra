//go:build windows

package collector

import (
	"context"
	"testing"
)

func TestGetInstalledApps_Integration(t *testing.T) {
	ctx := context.Background()
	apps, err := GetInstalledApps(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(apps) == 0 {
		t.Error("expected at least some installed apps")
	}

	for i, app := range apps {
		if app.Name == "" {
			t.Errorf("app %d has empty name", i)
		}
	}

	t.Logf("Found %d installed applications", len(apps))

	for i, app := range apps {
		if i >= 5 {
			break
		}
		t.Logf("  %s (v%s) - %s", app.Name, app.Version, app.Vendor)
	}
}

func TestGetInstalledApps_NoDuplicates(t *testing.T) {
	ctx := context.Background()
	apps, err := GetInstalledApps(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	seen := make(map[string]bool)
	for _, app := range apps {
		key := app.Name + "|" + app.Version
		if seen[key] {
			t.Logf("potential duplicate: %s v%s", app.Name, app.Version)
		}
		seen[key] = true
	}
}

func BenchmarkGetInstalledApps(b *testing.B) {
	ctx := context.Background()
	for b.Loop() {
		_, _ = GetInstalledApps(ctx)
	}
}
