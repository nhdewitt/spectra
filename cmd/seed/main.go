// Command seed inserts synthetic agents, current_metrics, and labels into a
// Spectra database for frontend/performance testing. It is a development tool,
// not part of the running system.
//
// Every seeded agent has a hostname prefixed with "seed-" so the whole fake
// fleet can be removed in one statement:
//
//	DELETE FROM agents WHERE hostname LIKE 'seed-%';
//
// (current_metrics and agent_labels cascade on agent delete)
//
// Safety: there is no default database URL, and -confirm is required. This makes
// seeding a deliberate act and avoids pointing it at production by accident.
//
// Usage:
//
//	seed -db "postgres://user:pass@host:5432/spectra?sslmode=disable" -n 500 -confirm
package main

import (
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"math/big"
	mrand "math/rand"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	hostnamePrefix = "seed-"
	fakeSecretHash = "seed-no-auth" // seeded agents never authenticate
)

func main() {
	var (
		dbURL   = flag.String("db", "", "PostgreSQL connection URL (required, no default)")
		n       = flag.Int("n", 100, "number of fake agents to insert (default: 100)")
		confirm = flag.Bool("confirm", false, "required: confirm you intend to write synthetic data")
		seed    = flag.Int64("seed", 0, "RNG seed for reproducible runs (default: 0 = time-based)")
		clean   = flag.Bool("clean", false, "delete all seed-* agents and exit (no insert)")
	)
	flag.Parse()

	if *dbURL == "" {
		fatal("the -db connection URL is required (no default)")
	}
	if !*confirm && !*clean {
		fatal("refusing to write without -confirm (use -clean to remove seed data)")
	}
	if *n < 1 && !*clean {
		fatal("-n must be at least 1")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, *dbURL)
	if err != nil {
		fatal("connect: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		fatal("ping: %v", err)
	}

	if *clean {
		tag, err := pool.Exec(ctx, `DELETE FROM agents WHERE hostname LIKE $1`, hostnamePrefix+"%")
		if err != nil {
			fatal("clean: %v", err)
		}
		fmt.Printf("Removed %d seed agents.\n", tag.RowsAffected())
		return
	}

	rng := mrand.New(mrand.NewSource(seedValue(*seed)))
	if err := run(ctx, pool, *n, rng); err != nil {
		fatal("seed: %v", err)
	}
	fmt.Printf("Inserted %d fake agents (hostname prefix %q).\n", *n, hostnamePrefix)
	fmt.Printf("Remove them with: seed -db <url> -clean\n")
}

type osProfile struct {
	os       string
	platform string
	arch     string
	cpuModel string
	hardware string
	weight   int // relative frequency
}

var osProfiles = []osProfile{
	{"linux", "Ubuntu 24.04", "amd64", "Intel Xeon E-2288G", "dell-r640", 50},
	{"linux", "Debian 12", "amd64", "AMD EPYC 7402P", "supermicro", 25},
	{"linux", "Raspberry Pi OS", "arm64", "BCM2711", "raspberry-pi", 12},
	{"windows", "Windows Server 2022", "amd64", "Intel Xeon Silver 4310", "vm", 8},
	{"freebsd", "FreeBSD 14.1", "amd64", "Intel Core i5-6500T", "vm", 5},
}

type status int

const (
	stOnline status = iota
	stWarn
	stCrit
	stStale
	stOffline
)

type statusWeight struct {
	s status
	w int
}

var statusWeights = []statusWeight{
	{stOnline, 85},
	{stWarn, 7},
	{stCrit, 3},
	{stStale, 3},
	{stOffline, 2},
}

var (
	envValues  = []string{"prod", "staging", "dev"}
	roleValues = []string{"web", "db", "cache", "worker", "lb"}
)

func run(ctx context.Context, pool *pgxpool.Pool, n int, rng *mrand.Rand) error {
	now := time.Now()

	for i := range n {
		prof := pickOSProfile(rng)
		st := pickStatus(rng)
		role := roleValues[rng.Intn(len(roleValues))]
		env := envValues[rng.Intn(len(envValues))]

		id := newUUID()
		hostname := fmt.Sprintf("%s%s-%04d", hostnamePrefix, role, i+1)
		lastSeen := lastSeenFor(st, now, rng)
		cores := cpuCoresFor(prof, rng)
		ramTotal := ramTotalFor(prof, rng)

		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin: %w", err)
		}

		// agents
		_, err = tx.Exec(ctx, `
		INSERT INTO agents
			(id, secret_hash, hostname, os, platform, arch, cpu_model, cpu_cores, ram_total, registered_at, last_seen)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11);`,
			id,
			fakeSecretHash,
			hostname,
			prof.os,
			prof.platform,
			prof.arch,
			prof.cpuModel,
			cores,
			ramTotal,
			now.Add(-time.Duration(rng.Intn(90*24))*time.Hour),
			lastSeen,
		)
		if err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("insert agent %s: %w", hostname, err)
		}

		// current_metrics
		cpu, ram, disk, temp := metricsFor(st, rng)
		_, err = tx.Exec(ctx, `
		INSERT INTO current_metrics
			(agent_id, cpu_usage, load_normalized, ram_percent, swap_percent, disk_max_percent, net_rx_bytes, net_tx_bytes,
			max_temp, uptime, process_count, reboot_required, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13);`,
			id,
			cpu,
			cpu/100.0*float64(cores),
			ram,
			rng.Float64()*15,
			disk,
			int64(rng.Intn(500_000_000_000)),
			int64(rng.Intn(200_000_000_000)),
			temp,
			int64(rng.Intn(90*24*3600)),
			20+rng.Intn(400),
			rng.Float64() < 0.05,
			lastSeen,
		)
		if err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("insert current_metrics %s: %w", hostname, err)
		}

		// labels
		labels := []struct {
			key, value, source string
		}{
			{"os", prof.os, "auto"},
			{"arch", prof.arch, "auto"},
			{"platform", prof.platform, "auto"},
			{"agent_version", "1.0.0", "auto"},
			{"env", env, "user"},
			{"role", role, "user"},
			{"hardware", prof.hardware, "user"},
		}
		for _, l := range labels {
			_, err = tx.Exec(ctx, `
			INSERT INTO agent_labels
				(agent_id, key, value, source, updated_at)
			VALUES ($1,$2,$3,$4,$5)
			ON CONFLICT (agent_id, key) DO NOTHING;`,
				id, l.key, l.value, l.source, now,
			)
			if err != nil {
				_ = tx.Rollback(ctx)
				return fmt.Errorf("insert label %s/%s: %w", hostname, l.key, err)
			}
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit %s: %w", hostname, err)
		}
	}
	return nil
}

func pickWeighted[T any](rng *mrand.Rand, items []T, weight func(T) int) T {
	total := 0
	for _, item := range items {
		total += weight(item)
	}

	if len(items) == 0 || total <= 0 {
		panic("pickWeighted: empty items or non-positive total weight")
	}

	r := rng.Intn(total)
	for _, item := range items {
		w := weight(item)
		if r < w {
			return item
		}
		r -= w
	}

	return items[0]
}

func pickOSProfile(rng *mrand.Rand) osProfile {
	return pickWeighted(rng, osProfiles, func(p osProfile) int {
		return p.weight
	})
}

func pickStatus(rng *mrand.Rand) status {
	sw := pickWeighted(rng, statusWeights, func(sw statusWeight) int {
		return sw.w
	})
	return sw.s
}

func lastSeenFor(st status, now time.Time, rng *mrand.Rand) time.Time {
	switch st {
	case stStale:
		return now.Add(-time.Duration(180+rng.Intn(360)) * time.Second)
	case stOffline:
		return now.Add(-time.Duration(11+rng.Intn(120)) * time.Minute)
	default:
		return now.Add(-time.Duration(rng.Intn(60)) * time.Second)
	}
}

// metricsFor returns cpu, ram, disk, temp correlated with status so the
// StatBar and status filters reflect the intended distribution. Stale/offline
// agents keep their last-known (low) metrics; status comes from last_seen.
func metricsFor(st status, rng *mrand.Rand) (cpu, ram, disk, temp float64) {
	randRange := func(min, max float64) float64 { return min + rng.Float64()*(max-min) }

	cpu = randRange(5, 45)
	ram = randRange(5, 45)
	disk = randRange(20, 75)

	switch st {
	case stWarn:
		switch rng.Intn(3) {
		case 0:
			cpu = randRange(80, 94)
		case 1:
			ram = randRange(80, 94)
		case 2:
			disk = randRange(98, 98.9)
		}
		temp = randRange(40, 65)

	case stCrit:
		switch rng.Intn(3) {
		case 0:
			cpu = randRange(95, 100)
		case 1:
			ram = randRange(95, 100)
		case 2:
			disk = randRange(99, 100)
		}
		temp = randRange(70, 90)

	default:
		temp = randRange(35, 65)
	}

	return
}

func cpuCoresFor(p osProfile, rng *mrand.Rand) int32 {
	if p.hardware == "raspberry-pi" {
		return 4
	}
	cores := [...]int32{4, 6, 8, 12, 16, 24, 32}
	return cores[rng.Intn(len(cores))]
}

func ramTotalFor(p osProfile, rng *mrand.Rand) int64 {
	if p.hardware == "raspberry-pi" {
		gb := [...]int64{2, 4, 8}[rng.Intn(3)]
		return gb * 1024 * 1024 * 1024
	}
	gb := [...]int64{16, 32, 64, 128}[rng.Intn(4)]
	return gb * 1024 * 1024 * 1024
}

func newUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func seedValue(s int64) int64 {
	if s != 0 {
		return s
	}
	max := big.NewInt(1 << 62)
	v, err := rand.Int(rand.Reader, max)
	if err != nil {
		fatal("generating seed: %v", err)
	}
	return v.Int64()
}

func fatal(format string, args ...any) {
	if !strings.HasSuffix(format, "\n") {
		format += "\n"
	}
	fmt.Fprintf(os.Stderr, "seed: "+format, args...)
	os.Exit(1)
}
