package util

import (
	"math"
	"testing"
	"time"
)

func TestPercent(t *testing.T) {
	tests := []struct {
		name        string
		used, total uint64
		want        float64
	}{
		{"Zero of Hundred", 0, 100, 0.0},
		{"Hundred of Hundred", 100, 100, 100.0},
		{"Fifty of Hundred", 50, 100, 50.0},
		{"Division by Zero", 0, 0, 0.0},
		{"Small Fraction", 1, 1000000, 0.0001},
		{"Over 100 Percent", 150, 100, 150.0},
		{"Large Values", 8000000000, 16000000000, 50.0},
		{"One of One", 1, 1, 100.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Percent(tt.used, tt.total)
			if math.Abs(got-tt.want) > 0.0001 {
				t.Errorf("Percent(%d, %d) = %f, want %f", tt.used, tt.total, got, tt.want)
			}
		})
	}
}

func TestPercent_Float(t *testing.T) {
	tests := []struct {
		name        string
		used, total float64
		want        float64
	}{
		{"Float Values", 25.5, 100.0, 25.5},
		{"Small Float", 0.001, 1.0, 0.1},
		{"Zero Total", 50.0, 0.0, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Percent(tt.used, tt.total)
			if math.Abs(got-tt.want) > 0.0001 {
				t.Errorf("Percent(%f, %f) = %f, want %f", tt.used, tt.total, got, tt.want)
			}
		})
	}
}

func TestRate(t *testing.T) {
	tests := []struct {
		name    string
		Delta   uint64
		seconds float64
		want    uint64
	}{
		{"Normal Rate", 1000, 1.0, 1000},
		{"Half Second", 1000, 0.5, 2000},
		{"Two Seconds", 1000, 2.0, 500},
		{"Zero Delta", 0, 1.0, 0},
		{"Zero Seconds", 1000, 0.0, 0},
		{"Negative Seconds", 1000, -1.0, 0},
		{"Large Values", 10000000000, 10.0, 1000000000},
		{"Small Interval", 100, 0.001, 100000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Rate(tt.Delta, tt.seconds)
			if got != tt.want {
				t.Errorf("Rate(%d, %f) = %d, want %d", tt.Delta, tt.seconds, got, tt.want)
			}
		})
	}
}

func TestDelta(t *testing.T) {
	tests := []struct {
		name       string
		curr, prev uint64
		want       uint64
	}{
		{"Normal Delta", 100, 50, 50},
		{"Zero Delta", 100, 100, 0},
		{"Counter Wraparound", 50, 100, 0}, // Returns 0 on wraparound
		{"Large Values", 10000000000, 5000000000, 5000000000},
		{"From Zero", 100, 0, 100},
		{"Both Zero", 0, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Delta(tt.curr, tt.prev)
			if got != tt.want {
				t.Errorf("Delta(%d, %d) = %d, want %d", tt.curr, tt.prev, got, tt.want)
			}
		})
	}
}

func TestMakeUintParser(t *testing.T) {
	t.Run("Valid Parsing", func(t *testing.T) {
		fields := []string{"123", "456", "789"}
		parse := MakeUintParser(fields, "test")

		if got := parse(0); got != 123 {
			t.Errorf("parse(0) = %d, want 123", got)
		}
		if got := parse(1); got != 456 {
			t.Errorf("parse(1) = %d, want 456", got)
		}
		if got := parse(2); got != 789 {
			t.Errorf("parse(2) = %d, want 789", got)
		}
	})

	t.Run("Invalid Value Returns Zero", func(t *testing.T) {
		fields := []string{"123", "invalid", "789"}
		parse := MakeUintParser(fields, "test")

		if got := parse(1); got != 0 {
			t.Errorf("parse(1) = %d, want 0 for invalid value", got)
		}
	})

	t.Run("Large Values", func(t *testing.T) {
		fields := []string{"18446744073709551615"} // Max uint64
		parse := MakeUintParser(fields, "test")

		if got := parse(0); got != 18446744073709551615 {
			t.Errorf("parse(0) = %d, want max uint64", got)
		}
	})

	t.Run("Negative Value Returns Zero", func(t *testing.T) {
		fields := []string{"-123"}
		parse := MakeUintParser(fields, "test")

		if got := parse(0); got != 0 {
			t.Errorf("parse(0) = %d, want 0 for negative value", got)
		}
	})
}

func TestValidateTimeDelta(t *testing.T) {
	t.Run("Valid Positive Delta", func(t *testing.T) {
		now := time.Now()
		last := now.Add(-5 * time.Second)
		got := ValidateTimeDelta(now, last, "test")
		if got < 4.9 || got > 5.1 {
			t.Errorf("ValidateTimeDelta = %f, want ~5.0", got)
		}
	})

	t.Run("Zero Delta", func(t *testing.T) {
		now := time.Now()
		got := ValidateTimeDelta(now, now, "test")
		if got != 0 {
			t.Errorf("ValidateTimeDelta = %f, want 0 for zero Delta", got)
		}
	})

	t.Run("Negative Delta", func(t *testing.T) {
		now := time.Now()
		future := now.Add(5 * time.Second)
		got := ValidateTimeDelta(now, future, "test")
		if got != 0 {
			t.Errorf("ValidateTimeDelta = %f, want 0 for negative Delta", got)
		}
	})

	t.Run("Small Delta", func(t *testing.T) {
		now := time.Now()
		last := now.Add(-100 * time.Millisecond)
		got := ValidateTimeDelta(now, last, "test")
		if got < 0.09 || got > 0.11 {
			t.Errorf("ValidateTimeDelta = %f, want ~0.1", got)
		}
	})
}

func TestIsDigit(t *testing.T) {
	tests := []struct {
		input byte
		want  bool
	}{
		{'0', true},
		{'1', true},
		{'5', true},
		{'9', true},
		{'a', false},
		{'z', false},
		{'A', false},
		{' ', false},
		{'.', false},
		{'-', false},
		{0, false},
		{255, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			got := IsDigit(tt.input)
			if got != tt.want {
				t.Errorf("IsDigit(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestCleanVendor(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Ubuntu Developers <ubuntu-devel@lists.ubuntu.com>", "Ubuntu Developers"},
		{"Simple Vendor", "Simple Vendor"},
		{"<only@email.com>", ""},
		{"", ""},
		{"  Spaced Vendor  <email@test.com>  ", "Spaced Vendor"},
		// FreeBSD pkg maintainer formats
		{"ports@FreeBSD.org", ""},
		{"jhixson@FreeBSD.org", ""},
		{"antoine@FreeBSD.org", ""},
		{"FreeBSD Ports <ports@FreeBSD.org>", "FreeBSD Ports"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := CleanVendor(tt.input)
			if got != tt.want {
				t.Errorf("CleanVendor(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func BenchmarkCleanVendor_WithEmail(b *testing.B) {
	input := "Ubuntu Developers <ubuntu-devel@lists.ubuntu.com>"
	b.ReportAllocs()
	for b.Loop() {
		_ = CleanVendor(input)
	}
}

func BenchmarkCleanVendor_Simple(b *testing.B) {
	input := "Microsoft Corporation"
	b.ReportAllocs()
	for b.Loop() {
		_ = CleanVendor(input)
	}
}
