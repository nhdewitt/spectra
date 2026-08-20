package collector

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func TestHasNonFinite(t *testing.T) {
	tests := []struct {
		name string
		v    any
		want bool
	}{
		{"clean cpu", &protocol.CPUMetric{Usage: 42, CoreUsage: []float64{1, 2}}, false},
		{"nan scalar", &protocol.CPUMetric{Usage: math.NaN()}, true},
		{"inf scalar", &protocol.CPUMetric{Usage: math.Inf(1)}, true},
		{"negative inf scalar", &protocol.CPUMetric{Usage: math.Inf(-1)}, true},
		{"nan in float slice", &protocol.CPUMetric{CoreUsage: []float64{1, math.NaN(), 3}}, true},
		{"empty float slice", &protocol.CPUMetric{CoreUsage: []float64{}}, false},
		{"nil float slice", &protocol.CPUMetric{}, false},
		{"nan behind a pointer field", &protocol.TemperatureMetric{Sensor: "test-sensor", Temp: 40, Max: ptr(math.NaN())}, true},
		{"nil pointer field", &protocol.TemperatureMetric{Sensor: "test-sensor", Temp: 40}, false},
		{"nan nested in a list element", &protocol.ProcessListMetric{Processes: []protocol.ProcessMetric{
			{Pid: 1, Name: "init"},
			{Pid: 2, Name: "test-proc", CPUPercent: math.NaN()},
		}}, true},
		{"clean list", &protocol.ProcessListMetric{Processes: []protocol.ProcessMetric{
			{Pid: 1, Name: "init", CPUPercent: 0.5},
		}}, false},
		{"metric with no floats at all", &protocol.SystemMetric{Uptime: 100, Processes: 40}, false},
		{"nil", nil, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := hasNonFinite(tc.v); got != tc.want {
				t.Errorf("hasNonFinite: got %v, want %v", got, tc.want)
			}
		})
	}
}

func ptr[T any](v T) *T { return &v }

func TestSend_DropsNonFiniteMetric(t *testing.T) {
	out := make(chan protocol.Envelope, 4)
	c := New("test-host", out, testLogger())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c.send(ctx, &protocol.CPUMetric{Usage: math.NaN()})

	select {
	case env := <-out:
		t.Fatalf("a non-finite metric reached the channel: %+v", env)
	default:
	}
}

func TestSend_PassesFiniteMetric(t *testing.T) {
	out := make(chan protocol.Envelope, 4)
	c := New("test-host", out, testLogger())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c.send(ctx, &protocol.CPUMetric{Usage: 42, CoreUsage: []float64{1, 2}})

	select {
	case env := <-out:
		if env.Type != "cpu" {
			t.Errorf("envelope type: got %q, want cpu", env.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("a finite metric never reached the channel")
	}
}

func TestReportNonFinite_WarnsOncePerMetricType(t *testing.T) {
	c := New("test-host", make(chan protocol.Envelope, 1), testLogger())

	c.reportNonFinite("cpu")
	c.reportNonFinite("cpu")
	c.reportNonFinite("temperature")

	if !c.reported["cpu"] || !c.reported["temperature"] {
		t.Error("both metric types should be marked reported")
	}
	if len(c.reported) != 2 {
		t.Errorf("reported types: got %d, want 2", len(c.reported))
	}
}
