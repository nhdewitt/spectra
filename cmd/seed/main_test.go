package main

import (
	"math"
	mrand "math/rand"
	"regexp"
	"testing"
)

const iters = 100_000

func fixedRNG() *mrand.Rand { return mrand.New(mrand.NewSource(1)) }

func TestPickOSProfileDistribution(t *testing.T) {
	rng := fixedRNG()

	counts := map[string]int{}
	for range iters {
		counts[pickOSProfile(rng).platform]++
	}

	total := 0
	for _, p := range osProfiles {
		total += p.weight
	}

	for _, p := range osProfiles {
		want := float64(p.weight) / float64(total)
		got := float64(counts[p.platform]) / float64(iters)
		if math.Abs(got-want) > 0.02 {
			t.Errorf("%s: freq = %.3f, want ~%.3f (weight %d)", p.platform, got, want, p.weight)
		}
	}
}

func TestPickStatusDistribution(t *testing.T) {
	rng := fixedRNG()

	counts := map[status]int{}
	for range iters {
		counts[pickStatus(rng)]++
	}

	total := 0
	for _, sw := range statusWeights {
		total += sw.w
	}

	for _, sw := range statusWeights {
		want := float64(sw.w) / float64(total)
		got := float64(counts[sw.s]) / float64(iters)
		if math.Abs(got-want) > 0.02 {
			t.Errorf("status %d: freq = %.3f, want ~%.3f (weight %d)", sw.s, got, want, sw.w)
		}
	}

	// Online must dominate
	if counts[stOnline] < counts[stWarn]+counts[stCrit]+counts[stStale]+counts[stOffline] {
		t.Errorf("online (%d) should exceed all unhealthy combined", counts[stOnline])
	}
}

func TestMetricsForBands(t *testing.T) {
	rng := fixedRNG()

	inBand := func(v, lo, hi float64) bool { return v >= lo && v <= hi }

	for range iters {
		// stWarn: exactly one of cpu/ram/disk in its warn band
		cpu, ram, disk, temp := metricsFor(stWarn, rng)
		warnHits := 0
		if inBand(cpu, 80, 94) {
			warnHits++
		}
		if inBand(ram, 80, 94) {
			warnHits++
		}
		if inBand(disk, 98, 98.9) {
			warnHits++
		}
		if warnHits != 1 {
			t.Fatalf("stWarn: %d metrics in warn band, want 1 (cpu=%.1f ram=%.1f disk=%.1f)", warnHits, cpu, ram, disk)
		}
		if !inBand(temp, 40, 65) {
			t.Errorf("stWarn temp = %.1f, want 40-65", temp)
		}

		// stCrit: exactly one in its crit band
		cpu, ram, disk, temp = metricsFor(stCrit, rng)
		critHits := 0
		if inBand(cpu, 95, 100) {
			critHits++
		}
		if inBand(ram, 95, 100) {
			critHits++
		}
		if inBand(disk, 99, 100) {
			critHits++
		}
		if critHits != 1 {
			t.Fatalf("stCrit: %d metrics in crit band, want 1 (cpu=%.1f ram=%.1f disk=%.1f)", critHits, cpu, ram, disk)
		}
		if !inBand(temp, 70, 90) {
			t.Errorf("stCrit temp = %.1f, want 70-90", temp)
		}

		// stOnline: all of cpu/ram/disk in the low band
		cpu, ram, disk, _ = metricsFor(stOnline, rng)
		if !inBand(cpu, 5, 45) {
			t.Errorf("stOnline cpu = %.1f, want 5-45", cpu)
		}
		if !inBand(ram, 5, 45) {
			t.Errorf("stOnline ram = %.1f, want 5-45", ram)
		}
		if !inBand(disk, 20, 75) {
			t.Errorf("stOnline disk = %.1f, want 20-75", disk)
		}
	}
}

func TestNewUUID(t *testing.T) {
	re := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

	const iters = 10_000
	seen := make(map[string]struct{}, iters)
	for range iters {
		u := newUUID()
		if !re.MatchString(u) {
			t.Fatalf("invalid v4 UUID format: %q", u)
		}
		if _, dup := seen[u]; dup {
			t.Fatalf("duplicate UUID generated: %q", u)
		}
		seen[u] = struct{}{}
	}
}
