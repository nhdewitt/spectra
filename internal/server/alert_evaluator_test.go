package server

import (
	"math"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nhdewitt/spectra/internal/database"
)

func TestServiceStatusHealthy(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   bool
	}{
		// Linux systemd
		{"systemd active", "active", true},
		{"systemd inactive", "inactive", false},
		{"systemd failed", "failed", false},
		// Darwin launchd
		{"launchd running", "running", true},
		{"launchd stopped", "stopped", false},
		// FreeBSD rc.d (enabled-state)
		{"rcd active", "active", true},
		{"rcd inactive", "inactive", false},
		// Windows SC (title-case live state)
		{"windows Running", "Running", true},
		{"windows Stopped", "Stopped", false},
		{"windows Paused", "Paused", false},
		// Windows transitional states must not fire
		{"windows StartPending", "StartPending", true},
		{"windows StopPending", "StopPending", true},
		{"windows ContinuePending", "ContinuePending", true},
		{"windows PausePending", "PausePending", true},
		// Hygiene
		{"uppercase RUNNING", "RUNNING", true},
		{"whitespace padded", "  running  ", true},
		{"empty", "", false},
		{"unknown", "Unknown", false},
		{"garbage", "florp", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := serviceStatusHealthy(tt.status); got != tt.want {
				t.Errorf("serviceStatusHealthy(%q) = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

func diskRow(t time.Time, pct float64) database.GetDiskTrendRow {
	return database.GetDiskTrendRow{
		Time:        pgtype.Timestamptz{Time: t, Valid: true},
		UsedPercent: pgtype.Float8{Float64: pct, Valid: true},
	}
}

func TestLinearProjectHours_Filling(t *testing.T) {
	base := time.Now().Add(-6 * time.Hour)
	// 1 percentage point per hour, starting at 50%. Should hit 100% in 50h
	// from the first sample; latest sample is at +5h (55%), so ~45h remaining.
	rows := []database.GetDiskTrendRow{
		diskRow(base, 50),
		diskRow(base.Add(1*time.Hour), 51),
		diskRow(base.Add(2*time.Hour), 52),
		diskRow(base.Add(3*time.Hour), 53),
		diskRow(base.Add(4*time.Hour), 54),
		diskRow(base.Add(5*time.Hour), 55),
	}

	hours, latest, err := linearProjectHours(rows)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(latest-55) > 0.001 {
		t.Errorf("latestPct = %v, want 55", latest)
	}
	if math.Abs(hours-45) > 0.01 {
		t.Errorf("hoursRemaining = %v, want ~45", hours)
	}
}

func TestLinearProjectHours_Flat(t *testing.T) {
	base := time.Now().Add(-6 * time.Hour)
	rows := []database.GetDiskTrendRow{
		diskRow(base, 60),
		diskRow(base.Add(1*time.Hour), 60),
		diskRow(base.Add(2*time.Hour), 60),
		diskRow(base.Add(3*time.Hour), 60),
	}
	hours, latest, err := linearProjectHours(rows)
	if err != nil {
		t.Fatal(err)
	}
	if hours != -1 {
		t.Errorf("flat slope hoursRemaining = %v, want -1", hours)
	}
	if math.Abs(latest-60) > 0.001 {
		t.Errorf("latestPct = %v, want 60", latest)
	}
}

func TestLinearProjectHours_Shrinking(t *testing.T) {
	base := time.Now().Add(-6 * time.Hour)
	rows := []database.GetDiskTrendRow{
		diskRow(base, 80),
		diskRow(base.Add(1*time.Hour), 78),
		diskRow(base.Add(2*time.Hour), 76),
		diskRow(base.Add(3*time.Hour), 74),
	}
	hours, _, err := linearProjectHours(rows)
	if err != nil {
		t.Fatal(err)
	}
	if hours != -1 {
		t.Errorf("shrinking slope hoursRemaining = %v, want -1", hours)
	}
}

func TestLinearProjectHours_SkipsInvalidRows(t *testing.T) {
	base := time.Now().Add(-6 * time.Hour)
	rows := []database.GetDiskTrendRow{
		diskRow(base, 50),
		{Time: pgtype.Timestamptz{Valid: false}, UsedPercent: pgtype.Float8{Float64: 999, Valid: true}}, // bad time
		diskRow(base.Add(1*time.Hour), 51),
		{Time: pgtype.Timestamptz{Time: base.Add(2 * time.Hour), Valid: true}, UsedPercent: pgtype.Float8{Valid: false}}, // bad pct
		diskRow(base.Add(3*time.Hour), 53),
	}
	// Valid points: (0,50),(1,51),(3,53) — slope 1/h, latest 53, ~47h to 100.
	hours, latest, err := linearProjectHours(rows)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(latest-53) > 0.001 {
		t.Errorf("latestPct = %v, want 53", latest)
	}
	if math.Abs(hours-47) > 0.01 {
		t.Errorf("hoursRemaining = %v, want ~47", hours)
	}
}

func TestLinearProjectHours_InsufficientData(t *testing.T) {
	base := time.Now()
	rows := []database.GetDiskTrendRow{diskRow(base, 50)}
	if _, _, err := linearProjectHours(rows); err == nil {
		t.Error("expected error for single data point, got nil")
	}
}

func TestLinearProjectHours_DegenerateSameTime(t *testing.T) {
	now := time.Now()
	rows := []database.GetDiskTrendRow{
		diskRow(now, 50),
		diskRow(now, 60),
		diskRow(now, 70),
	}
	if _, _, err := linearProjectHours(rows); err == nil {
		t.Error("expected error for all-same-timestamp samples, got nil")
	}
}
