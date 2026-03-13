//go:build darwin

package inventory

import (
	"context"
	"testing"
)

func TestParseSystemProfiler(t *testing.T) {
	sample := []byte(`{
	"SPApplicationsDataType": [
		{
			"_name": "Safari",
			"version": "18.3",
			"obtained_from": "apple",
			"path": "/Applications/Safari.app"
		},
		{
			"_name": "Slack",
			"version": "4.36.140",
			"obtained_from": "mac_app_store",
			"path": "/Applications/Slack.app"
		},
		{
			"_name": "Visual Studio Code",
			"version": "1.87.0",
			"obtained_from": "identified_developer",
			"path": "/Applications/Visual Studio Code.app"
		},
		{
			"_name": "TestApp",
			"version": "1.0",
			"obtained_from": "unknown",
			"path": "/Applications/TestApp.app"
		}
	]
}`)

	apps, err := parseSystemProfiler(sample)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(apps) != 4 {
		t.Fatalf("expected 4 apps, got %d", len(apps))
	}

	tests := []struct {
		name, version, vendor string
	}{
		{"Safari", "18.3", "Apple"},
		{"Slack", "4.36.140", "Mac App Store"},
		{"Visual Studio Code", "1.87.0", "Identified Developer"},
		{"TestApp", "1.0", "Unknown"},
	}

	for i, tc := range tests {
		if apps[i].Name != tc.name {
			t.Errorf("apps[%d].Name = %q, want %q", i, apps[i].Name, tc.name)
		}
		if apps[i].Version != tc.version {
			t.Errorf("apps[%d].Version = %q, want %q", i, apps[i].Version, tc.version)
		}
		if apps[i].Vendor != tc.vendor {
			t.Errorf("apps[%d].Vendor = %q, want %q", i, apps[i].Vendor, tc.vendor)
		}
	}
}

func TestParseSystemProfiler_Empty(t *testing.T) {
	sample := []byte(`{"SPApplicationsDataType": []}`)

	apps, err := parseSystemProfiler(sample)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(apps) != 0 {
		t.Errorf("expected 0 apps, got %d", len(apps))
	}
}

func TestParseSystemProfiler_MissingFields(t *testing.T) {
	sample := []byte(`{
	"SPApplicationsDataType": [
		{
			"_name": "Test App"
		}
	]
}`)

	apps, err := parseSystemProfiler(sample)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(apps) != 1 {
		t.Fatalf("epxected 1 app, got %d", len(apps))
	}
	if apps[0].Name != "Test App" {
		t.Errorf("Name = %q, want 'Test App'", apps[0].Name)
	}
	if apps[0].Version != "" {
		t.Errorf("Version = %q, want empty", apps[0].Version)
	}
	if apps[0].Vendor != "" {
		t.Errorf("Vendor = %q, want empty", apps[0].Vendor)
	}
}

func TestParseSystemProfiler_InvalidJSON(t *testing.T) {
	_, err := parseSystemProfiler([]byte("invalid json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestGetInstalledApps_Integration(t *testing.T) {
	ctx := context.Background()
	apps, err := GetInstalledApps(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(apps) == 0 {
		t.Fatal("expected at least one installed app")
	}

	var foundSystemSettings bool
	for _, a := range apps {
		if a.Name == "System Settings" {
			foundSystemSettings = true
			t.Logf("System Settings: version=%s vendor=%s", a.Version, a.Vendor)
		}
	}

	if !foundSystemSettings {
		t.Error("Safari not found in installed apps")
	}

	t.Logf("found %d installed apps", len(apps))
	t.Log("first 5 apps:")
	for i, a := range apps {
		if i >= 5 {
			break
		}
		t.Logf("%s: version=%s vendor=%s", a.Name, a.Version, a.Vendor)
	}
}
