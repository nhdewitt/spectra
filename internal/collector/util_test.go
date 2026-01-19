package collector

import (
	"math"
	"testing"
)

func TestPercent(t *testing.T) {
	tests := []struct {
		used, total uint64
		want        float64
	}{
		{0, 100, 0.0},
		{100, 100, 100.0},
		{50, 100, 50.0},
		{0, 0, 0.0}, // Division by zero
		{1, 1000000, 0.0001},
	}

	for _, tt := range tests {
		got := percent(tt.used, tt.total)
		if math.Abs(got-tt.want) > 0.0001 {
			t.Errorf("percent(%d, %d) = %f, want %f", tt.used, tt.total, got, tt.want)
		}
	}
}
