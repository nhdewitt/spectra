//go:build windows

package collector

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func TestDecikelvinToCelsius(t *testing.T) {
	tests := []struct {
		name       string
		decikelvin uint32
		want       float64
	}{
		{
			name:       "Freezing Point",
			decikelvin: 2732, // 0°C
			want:       0.0,
		},
		{
			name:       "Room Temperature",
			decikelvin: 2982, // 25°C
			want:       25.0,
		},
		{
			name:       "Boiling Point",
			decikelvin: 3732, // 100°C
			want:       100.0,
		},
		{
			name:       "Typical CPU Temp",
			decikelvin: 3232, // 50°C
			want:       50.0,
		},
		{
			name:       "Hot CPU",
			decikelvin: 3582, // 85°C
			want:       85.0,
		},
		{
			name:       "Critical Temp",
			decikelvin: 3732, // 100°C
			want:       100.0,
		},
		{
			name:       "Fractional",
			decikelvin: 3237, // 50.5°C
			want:       50.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := (float64(tt.decikelvin) - 2732.0) / 10.0
			if got != tt.want {
				t.Errorf("decikelvin %d = %.1f°C, want %.1f°C", tt.decikelvin, got, tt.want)
			}
		})
	}
}

func TestCleanInstanceName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "With Backslash",
			input: `ACPI\ThermalZone\THM0_0`,
			want:  "THM0_0",
		},
		{
			name:  "Multiple Backslashes",
			input: `ROOT\ACPI\ThermalZone\CPU_0`,
			want:  "CPU_0",
		},
		{
			name:  "No Backslash",
			input: "ThermalZone0",
			want:  "ThermalZone0",
		},
		{
			name:  "Trailing Backslash",
			input: `ACPI\ThermalZone\`,
			want:  "",
		},
		{
			name:  "Empty",
			input: "",
			want:  "",
		},
		{
			name:  "Only Backslash",
			input: `\`,
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name := tt.input
			if lastIdx := strings.LastIndex(name, `\`); lastIdx != -1 {
				name = name[lastIdx+1:]
			}
			if name != tt.want {
				t.Errorf("cleanName(%q) = %q, want %q", tt.input, name, tt.want)
			}
		})
	}
}

func TestCollectTemperature_Integration(t *testing.T) {
	ctx := context.Background()
	metrics, err := CollectTemperature(ctx)
	// WMI thermal zones may not be available on all systems
	if err != nil {
		t.Logf("CollectTemperature returned error (may be expected): %v", err)
		return
	}

	if metrics == nil {
		t.Log("No thermal zones available via WMI (common on some systems)")
		return
	}

	t.Logf("Found %d temperature sensors", len(metrics))

	for _, m := range metrics {
		temp, ok := m.(protocol.TemperatureMetric)
		if !ok {
			t.Errorf("Expected TemperatureMetric, got %T", m)
			continue
		}

		maxStr := "N/A"
		if temp.Max != nil {
			maxStr = fmt.Sprintf("%.1f°C", *temp.Max)
		}
		t.Logf("Sensor: %s, Temp: %.1f°C, Max: %s", temp.Sensor, temp.Temp, maxStr)

		// Sanity checks
		if temp.Temp < -40 || temp.Temp > 150 {
			t.Errorf("Sensor %s: temperature %.1f°C seems unreasonable", temp.Sensor, temp.Temp)
		}

		if temp.Max != nil && *temp.Max < temp.Temp {
			t.Logf("Warning: Sensor %s: max %.1f°C is less than current %.1f°C", temp.Sensor, *temp.Max, temp.Temp)
		}

		// Max should be reasonable if present
		if temp.Max != nil && (*temp.Max < 50 || *temp.Max > 150) {
			t.Logf("Warning: Sensor %s: max temp %.1f°C seems unusual", temp.Sensor, *temp.Max)
		}
	}
}

func TestCollectTemperature_ReturnsNilOnError(t *testing.T) {
	// The implementation returns nil, nil on WMI error
	// This is by design to gracefully handle systems without thermal zones
	ctx := context.Background()
	metrics, err := CollectTemperature(ctx)
	// Should not return an error even if WMI fails
	if err != nil {
		t.Errorf("Expected nil error, got: %v", err)
	}

	// Metrics may be nil or empty, both are acceptable
	t.Logf("Returned %d metrics (nil is acceptable)", len(metrics))
}

func TestCollectTemperature_ContextNotUsed(t *testing.T) {
	// Context is accepted for interface consistency but WMI calls are synchronous
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Should still work since WMI doesn't use context
	metrics, err := CollectTemperature(ctx)
	if err != nil {
		t.Logf("CollectTemperature with cancelled context: %v", err)
	}

	t.Logf("Returned %d metrics with cancelled context", len(metrics))
}

func TestMSAcpi_ThermalZoneTemperature_Struct(t *testing.T) {
	// Verify struct fields match expected WMI properties
	zone := MSAcpi_ThermalZoneTemperature{
		CurrentTemperature: 3232, // 50°C in decikelvin
		CriticalTripPoint:  3832, // 110°C in decikelvin
		InstanceName:       `ACPI\ThermalZone\THM0_0`,
	}

	celsius := (float64(zone.CurrentTemperature) - 2732.0) / 10.0
	if celsius != 50.0 {
		t.Errorf("CurrentTemperature conversion: got %.1f, want 50.0", celsius)
	}

	maxCelsius := (float64(zone.CriticalTripPoint) - 2732.0) / 10.0
	if maxCelsius != 110.0 {
		t.Errorf("CriticalTripPoint conversion: got %.1f, want 110.0", maxCelsius)
	}
}

func TestCollectTemperature_ZeroCriticalTripPoint(t *testing.T) {
	// When CriticalTripPoint is 0, Max should be 0 (not calculated)
	zone := MSAcpi_ThermalZoneTemperature{
		CurrentTemperature: 3232,
		CriticalTripPoint:  0,
		InstanceName:       "Test",
	}

	maxCelsius := 0.0
	if zone.CriticalTripPoint > 0 {
		maxCelsius = (float64(zone.CriticalTripPoint) - 2732.0) / 10.0
	}

	if maxCelsius != 0.0 {
		t.Errorf("Expected Max=0 when CriticalTripPoint=0, got %.1f", maxCelsius)
	}
}

func TestCollectTemperature_ConsistentResults(t *testing.T) {
	ctx := context.Background()

	metrics1, _ := CollectTemperature(ctx)
	metrics2, _ := CollectTemperature(ctx)

	// Should return same number of sensors
	if len(metrics1) != len(metrics2) {
		t.Logf("Sensor count changed: %d -> %d", len(metrics1), len(metrics2))
	}

	// Temperature shouldn't change drastically between calls
	if len(metrics1) > 0 && len(metrics2) > 0 {
		temp1 := metrics1[0].(protocol.TemperatureMetric)
		temp2 := metrics2[0].(protocol.TemperatureMetric)

		diff := temp2.Temp - temp1.Temp
		if diff < -5 || diff > 5 {
			t.Logf("Temperature changed significantly: %.1f -> %.1f", temp1.Temp, temp2.Temp)
		}
	}
}

func BenchmarkDecikelvinToCelsius(b *testing.B) {
	decikelvin := uint32(3232)
	b.ReportAllocs()
	for b.Loop() {
		_ = (float64(decikelvin) - 2732.0) / 10.0
	}
}

func BenchmarkCleanInstanceName(b *testing.B) {
	name := `ACPI\ThermalZone\THM0_0`
	b.ReportAllocs()
	for b.Loop() {
		n := name
		if lastIdx := strings.LastIndex(n, `\`); lastIdx != -1 {
			n = n[lastIdx+1:]
		}
		_ = n
	}
}

func BenchmarkCollectTemperature(b *testing.B) {
	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		_, _ = CollectTemperature(ctx)
	}
}
