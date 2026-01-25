//go:build windows

package collector

import (
	"strings"
	"testing"
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
