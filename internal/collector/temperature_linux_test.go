//go:build linux

package collector

import (
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func TestParseThermalValueFrom(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    float64
		wantErr bool
	}{
		{
			name:  "Standard Temperature",
			input: "45000",
			want:  45.0,
		},
		{
			name:  "High Temperature",
			input: "85000",
			want:  85.0,
		},
		{
			name:  "Low Temperature",
			input: "20000",
			want:  20.0,
		},
		{
			name:  "Fractional",
			input: "45500",
			want:  45.5,
		},
		{
			name:  "Zero",
			input: "0",
			want:  0.0,
		},
		{
			name:  "With Whitespace",
			input: "  45000\n",
			want:  45.0,
		},
		{
			name:    "Empty",
			input:   "",
			wantErr: true,
		},
		{
			name:    "Only Whitespace",
			input:   "   \n\t   ",
			wantErr: true,
		},
		{
			name:    "Invalid",
			input:   "not_a_number",
			wantErr: true,
		},
		{
			name:  "Negative Temperature",
			input: "-5000",
			want:  -5.0,
		},
		{
			name:  "Very High Temperature",
			input: "125000",
			want:  125.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(tt.input)
			got, err := parseThermalValueFrom(r)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if got != tt.want {
				t.Errorf("parseThermalValueFrom() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseThermalZoneFrom(t *testing.T) {
	tests := []struct {
		name     string
		typeData string
		tempData string
		maxData  string
		hasMax   bool
		want     *protocol.TemperatureMetric
		wantErr  bool
	}{
		{
			name:     "Standard Zone",
			typeData: "x86_pkg_temp",
			tempData: "45000",
			maxData:  "100000",
			hasMax:   true,
			want: &protocol.TemperatureMetric{
				Sensor: "x86_pkg_temp",
				Temp:   45.0,
				Max:    100.0,
			},
		},
		{
			name:     "Without Max",
			typeData: "acpitz",
			tempData: "55000",
			hasMax:   false,
			want: &protocol.TemperatureMetric{
				Sensor: "acpitz",
				Temp:   55.0,
				Max:    0.0,
			},
		},
		{
			name:     "CPU Zone",
			typeData: "cpu-thermal",
			tempData: "62500",
			maxData:  "95000",
			hasMax:   true,
			want: &protocol.TemperatureMetric{
				Sensor: "cpu-thermal",
				Temp:   62.5,
				Max:    95.0,
			},
		},
		{
			name:     "GPU Zone",
			typeData: "gpu-thermal",
			tempData: "70000",
			maxData:  "105000",
			hasMax:   true,
			want: &protocol.TemperatureMetric{
				Sensor: "gpu-thermal",
				Temp:   70.0,
				Max:    105.0,
			},
		},
		{
			name:     "With Whitespace in Type",
			typeData: "  x86_pkg_temp\n",
			tempData: "45000",
			hasMax:   false,
			want: &protocol.TemperatureMetric{
				Sensor: "x86_pkg_temp",
				Temp:   45.0,
				Max:    0.0,
			},
		},
		{
			name:     "Invalid Max Ignored",
			typeData: "acpitz",
			tempData: "55000",
			maxData:  "invalid",
			hasMax:   true,
			want: &protocol.TemperatureMetric{
				Sensor: "acpitz",
				Temp:   55.0,
				Max:    0.0, // Invalid max is ignored
			},
		},
		{
			name:     "Invalid Temp",
			typeData: "acpitz",
			tempData: "invalid",
			hasMax:   false,
			wantErr:  true,
		},
		{
			name:     "Empty Type",
			typeData: "",
			tempData: "45000",
			hasMax:   false,
			want: &protocol.TemperatureMetric{
				Sensor: "",
				Temp:   45.0,
				Max:    0.0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			typeR := strings.NewReader(tt.typeData)
			tempR := strings.NewReader(tt.tempData)

			var maxR io.Reader
			if tt.hasMax {
				maxR = strings.NewReader(tt.maxData)
			}

			got, err := parseThermalZoneFrom(typeR, tempR, maxR)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if got.Sensor != tt.want.Sensor {
				t.Errorf("Sensor = %q, want %q", got.Sensor, tt.want.Sensor)
			}
			if got.Temp != tt.want.Temp {
				t.Errorf("Temp = %v, want %v", got.Temp, tt.want.Temp)
			}
			if got.Max != tt.want.Max {
				t.Errorf("Max = %v, want %v", got.Max, tt.want.Max)
			}
		})
	}
}

func TestCollectTemperature_Integration(t *testing.T) {
	// Check if thermal zones exist
	zones, err := filepath.Glob("/sys/class/thermal/thermal_zone*")
	if err != nil {
		t.Fatalf("Failed to glob thermal zones: %v", err)
	}

	if len(zones) == 0 {
		t.Skip("No thermal zones available on this system")
	}

	ctx := context.Background()
	metrics, err := CollectTemperature(ctx)
	if err != nil {
		t.Fatalf("CollectTemperature failed: %v", err)
	}

	t.Logf("Found %d temperature sensors", len(metrics))

	for _, m := range metrics {
		temp, ok := m.(protocol.TemperatureMetric)
		if !ok {
			t.Errorf("Expected TemperatureMetric, got %T", m)
			continue
		}

		t.Logf("Sensor: %s, Temp: %.1f°C, Max: %.1f°C", temp.Sensor, temp.Temp, temp.Max)

		// Sanity checks
		if temp.Temp < -40 || temp.Temp > 150 {
			t.Errorf("Sensor %s: temperature %.1f°C seems unreasonable", temp.Sensor, temp.Temp)
		}

		if temp.Max != 0 && temp.Max < temp.Temp {
			t.Errorf("Sensor %s: max %.1f°C is less than current %.1f°C", temp.Sensor, temp.Max, temp.Temp)
		}
	}
}

func TestCollectTemperature_NoZones(t *testing.T) {
	// This test verifies behavior when no zones match
	// In practice, most Linux systems have at least one thermal zone
	zones, _ := filepath.Glob("/sys/class/thermal/thermal_zone*")
	if len(zones) == 0 {
		ctx := context.Background()
		metrics, err := CollectTemperature(ctx)
		if err != nil {
			t.Fatalf("CollectTemperature failed: %v", err)
		}
		if len(metrics) != 0 {
			t.Errorf("Expected 0 metrics, got %d", len(metrics))
		}
	} else {
		t.Skip("System has thermal zones - cannot test empty case")
	}
}

func TestReadThermalZone_Integration(t *testing.T) {
	zones, err := filepath.Glob("/sys/class/thermal/thermal_zone*")
	if err != nil || len(zones) == 0 {
		t.Skip("No thermal zones available")
	}

	// Test first available zone
	zone := zones[0]
	m, err := readThermalZone(zone)
	if err != nil {
		t.Fatalf("readThermalZone(%s) failed: %v", zone, err)
	}

	t.Logf("Zone %s: Sensor=%s, Temp=%.1f°C, Max=%.1f°C", zone, m.Sensor, m.Temp, m.Max)

	if m.Sensor == "" {
		t.Error("Sensor name is empty")
	}
}

func TestReadThermalZone_InvalidPath(t *testing.T) {
	_, err := readThermalZone("/nonexistent/path")
	if err == nil {
		t.Error("Expected error for invalid path")
	}
}

func TestCollectTemperature_ContextNotUsed(t *testing.T) {
	// Context is not currently used in the implementation
	// but the function signature accepts it for consistency
	zones, _ := filepath.Glob("/sys/class/thermal/thermal_zone*")
	if len(zones) == 0 {
		t.Skip("No thermal zones available")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Should still work since context isn't used for file reads
	metrics, err := CollectTemperature(ctx)
	if err != nil {
		t.Fatalf("CollectTemperature failed with cancelled context: %v", err)
	}

	if len(metrics) == 0 {
		t.Error("Expected metrics even with cancelled context")
	}
}

func TestParseThermalValueFrom_Precision(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"45000", 45.0},
		{"45100", 45.1},
		{"45010", 45.01},
		{"45001", 45.001},
		{"45555", 45.555},
	}

	for _, tt := range tests {
		r := strings.NewReader(tt.input)
		got, err := parseThermalValueFrom(r)
		if err != nil {
			t.Errorf("parseThermalValueFrom(%s) error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("parseThermalValueFrom(%s) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeMax(t *testing.T) {
	tests := []struct {
		name string
		temp float64
		max  float64
		want float64
	}{
		{
			name: "Valid max above temp",
			temp: 45.0,
			max:  95.0,
			want: 95.0,
		},
		{
			name: "Max equal to temp",
			temp: 60.0,
			max:  60.0,
			want: 60.0,
		},
		{
			name: "Max slightly above temp",
			temp: 59.5,
			max:  60.0,
			want: 60.0,
		},
		{
			name: "Max below temp",
			temp: 70.0,
			max:  65.0,
			want: 0.0,
		},
		{
			name: "Max zero (unset)",
			temp: 40.0,
			max:  0.0,
			want: 0.0,
		},
		{
			name: "Max negative (bogus)",
			temp: 40.0,
			max:  -274.0,
			want: 0.0,
		},
		{
			name: "Max extremely high",
			temp: 40.0,
			max:  500.0,
			want: 0.0,
		},
		{
			name: "Max just below upper bound",
			temp: 40.0,
			max:  199.9,
			want: 199.9,
		},
		{
			name: "Max exactly at upper bound",
			temp: 40.0,
			max:  200.0,
			want: 0.0,
		},
		{
			name: "Temp negative but max valid",
			temp: -10.0,
			max:  80.0,
			want: 80.0,
		},
		{
			name: "Both temp and max negative",
			temp: -20.0,
			max:  -10.0,
			want: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeMax(tt.temp, tt.max)
			if got != tt.want {
				t.Errorf(
					"normalizeMax(temp=%.1f, max=%.1f) = %.1f, want %.1f",
					tt.temp, tt.max, got, tt.want,
				)
			}
		})
	}
}

func BenchmarkParseThermalValueFrom(b *testing.B) {
	input := "45000"
	b.ReportAllocs()
	for b.Loop() {
		r := strings.NewReader(input)
		_, _ = parseThermalValueFrom(r)
	}
}

func BenchmarkParseThermalZoneFrom(b *testing.B) {
	typeData := "x86_pkg_temp"
	tempData := "45000"
	maxData := "100000"

	b.ReportAllocs()
	for b.Loop() {
		typeR := strings.NewReader(typeData)
		tempR := strings.NewReader(tempData)
		maxR := strings.NewReader(maxData)
		_, _ = parseThermalZoneFrom(typeR, tempR, maxR)
	}
}

func BenchmarkParseThermalZoneFrom_NoMax(b *testing.B) {
	typeData := "acpitz"
	tempData := "55000"

	b.ReportAllocs()
	for b.Loop() {
		typeR := strings.NewReader(typeData)
		tempR := strings.NewReader(tempData)
		_, _ = parseThermalZoneFrom(typeR, tempR, nil)
	}
}

func BenchmarkReadThermalZone(b *testing.B) {
	zones, err := filepath.Glob("/sys/class/thermal/thermal_zone*")
	if err != nil || len(zones) == 0 {
		b.Skip("No thermal zones available")
	}

	zone := zones[0]
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = readThermalZone(zone)
	}
}

func BenchmarkCollectTemperature(b *testing.B) {
	zones, _ := filepath.Glob("/sys/class/thermal/thermal_zone*")
	if len(zones) == 0 {
		b.Skip("No thermal zones available")
	}

	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		_, _ = CollectTemperature(ctx)
	}
}

func BenchmarkFilepathGlob_ThermalZones(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_, _ = filepath.Glob("/sys/class/thermal/thermal_zone*")
	}
}
