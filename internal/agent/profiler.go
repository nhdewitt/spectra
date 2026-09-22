//go:build spectraprof

package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"runtime/metrics"
	"runtime/pprof"
	"strconv"
	"time"

	"github.com/nhdewitt/spectra/internal/hostinfo"
	"github.com/nhdewitt/spectra/internal/protocol"
	"github.com/nhdewitt/spectra/internal/version"
)

// Configured by environment rather than by Config, so the agent's on-disk config
// schema is identical in both builds and a profiling host needs no file changes
// to go back to a release binary.
const (
	envProfileURL      = "SPECTRA_PPROF_URL"
	envProfileInterval = "SPECTRA_PPROF_INTERVAL"
	envProfileCPUSecs  = "SPECTRA_PPROF_CPU_SECONDS"
)

const (
	defaultProfileInterval = 5 * time.Minute
	defaultCPUSeconds      = 30
	uploadTimeout          = 30 * time.Second
	maxCPUSeconds          = 120
)

// profileUpload is one captured profile plus the host facts needed to compare it
// against another agent's. pprof carries none of this itself, and sample counts
// are not comparable across CPUs without it.
type profileUpload struct {
	AgentID  string `json:"agent_id"`
	Hostname string `json:"hostname"`

	OS       string `json:"os"`
	Arch     string `json:"arch"`
	GoARM    string `json:"goarm,omitempty"`
	CPUModel string `json:"cpu_model"`
	CPUCores int    `json:"cpu_cores"`

	RAMTotalBytes uint64 `json:"ram_total_bytes"`

	AgentVersion string `json:"agent_version"`
	AgentCommit  string `json:"agent_commit"`
	GoVersion    string `json:"go_version"`

	CGO bool `json:"cgo"`

	Kind        string    `json:"kind"`
	CapturedAt  time.Time `json:"captured_at"`
	DurationSec int       `json:"duration_sec"`
	Profile     []byte    `json:"profile"`

	Runtime runtimeSnapshot `json:"runtime"`
}

// runtimeSnapshot is the scalar state a pprof profile does not carry.
type runtimeSnapshot struct {
	GOMAXPROCS int `json:"gomaxprocs"`

	GOGC       int64 `json:"gogc"`
	GOMEMLIMIT int64 `json:"gomemlimit"`

	HeapObjectsBytes uint64 `json:"heap_objects_bytes"`
	HeapLiveBytes    uint64 `json:"heap_live_bytes"`
	TotalBytes       uint64 `json:"total_bytes"`

	GCCycles    uint64  `json:"gc_cycles"`
	GCCPUSecs   float64 `json:"gc_cpu_seconds"`
	TotalCPUSec float64 `json:"total_cpu_seconds"`

	Goroutines uint64 `json:"goroutines"`
}

// Read generically so an unsupported name on some future toolchain leaves
// a zero rather than failing the capture.
const (
	mHeapObjects = "/memory/classes/heap/objects:bytes"
	mTotalBytes  = "/memory/classes/total:bytes"
	mHeapLive    = "/gc/heap/live:bytes"
	mGCCycles    = "/gc/cycles/total:gc-cycles"
	mGOGC        = "/gc/gogc:percent"
	mGOMEMLIMIT  = "/gc/gomemlimit:bytes"
	mGCCPU       = "/cpu/classes/gc/total:cpu-seconds"
	mTotalCPU    = "/cpu/classes/total:cpu-seconds"
	mGoroutines  = "/sched/goroutines:goroutines"
)

func collectRuntimeSnapshot() runtimeSnapshot {
	names := []string{
		mHeapObjects, mTotalBytes, mHeapLive, mGCCycles,
		mGOGC, mGOMEMLIMIT, mGCCPU, mTotalCPU, mGoroutines,
	}

	samples := make([]metrics.Sample, len(names))
	for i, n := range names {
		samples[i].Name = n
	}
	metrics.Read(samples)

	byName := make(map[string]metrics.Value, len(samples))
	for _, s := range samples {
		byName[s.Name] = s.Value
	}

	u64 := func(name string) uint64 {
		v, ok := byName[name]
		if !ok || v.Kind() != metrics.KindUint64 {
			return 0
		}
		return v.Uint64()
	}
	f64 := func(name string) float64 {
		v, ok := byName[name]
		if !ok || v.Kind() != metrics.KindFloat64 {
			return 0
		}
		return v.Float64()
	}

	return runtimeSnapshot{
		GOMAXPROCS:       runtime.GOMAXPROCS(0),
		GOGC:             int64(u64(mGOGC)),
		GOMEMLIMIT:       int64(u64(mGOMEMLIMIT)),
		HeapObjectsBytes: u64(mHeapObjects),
		HeapLiveBytes:    u64(mHeapLive),
		TotalBytes:       u64(mTotalBytes),
		GCCycles:         u64(mGCCycles),
		GCCPUSecs:        f64(mGCCPU),
		TotalCPUSec:      f64(mTotalCPU),
		Goroutines:       u64(mGoroutines),
	}
}

func envDuration(key string, def time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return def
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return def
	}
	return d
}

func envInt(key string, def, max int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 || n > max {
		return def
	}
	return n
}

// startProfiler captures profiles on an interval and pushes them to the
// aggregator. With no aggregator configured, it returns immediately, so a
// profiling binary with no URL set behaves like a release one.
func (a *Agent) startProfiler(ctx context.Context) {
	url := os.Getenv(envProfileURL)
	if url == "" {
		a.Logger.Info("profiler build in but no aggregator configured", "env", envProfileURL)
		return
	}

	interval := envDuration(envProfileInterval, defaultProfileInterval)
	cpuSecs := envInt(envProfileCPUSecs, defaultCPUSeconds, maxCPUSeconds)

	if time.Duration(cpuSecs)*time.Second >= interval {
		a.Logger.Warn("cpu profile not shorter than interval; using default", "cpu_seconds", cpuSecs, "interval", interval)
		cpuSecs = defaultCPUSeconds
	}

	host := hostinfo.CollectHostInfo()
	client := &http.Client{Timeout: uploadTimeout}

	a.Logger.Info("profiler started", "url", url, "interval", interval, "cpu_seconds", cpuSecs)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		a.captureCycle(ctx, client, url, host, cpuSecs)

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// captureCycle takes one round of profiles. A failure in any one of them is logged and skipped
// rather than ending the loop.
func (a *Agent) captureCycle(ctx context.Context, client *http.Client, url string, host protocol.HostInfo, cpuSecs int) {
	cpu, err := captureCPU(ctx, cpuSecs)
	if err != nil {
		a.Logger.Warn("cpu profile failed", "error", err)
	} else {
		a.push(ctx, client, url, host, "cpu", cpuSecs, cpu)
	}

	if ctx.Err() != nil {
		return
	}

	for _, name := range []string{"heap", "goroutine"} {
		data, err := captureNamed(name)
		if err != nil {
			a.Logger.Warn("profile failed", "kind", name, "error", err)
			continue
		}
		a.push(ctx, client, url, host, name, 0, data)
	}
}

func (a *Agent) push(ctx context.Context, client *http.Client, url string, host protocol.HostInfo, kind string, durationSec int, data []byte) {
	up := profileUpload{
		AgentID:       a.Identity.ID,
		Hostname:      host.Hostname,
		OS:            host.OS,
		Arch:          host.Arch,
		GoARM:         version.GoARM,
		CPUModel:      host.CPUModel,
		CPUCores:      host.CPUCores,
		RAMTotalBytes: host.RAMTotal,
		AgentVersion:  version.Version,
		AgentCommit:   version.Commit,
		GoVersion:     runtime.Version(),
		CGO:           cgoEnabled,
		Kind:          kind,
		CapturedAt:    time.Now().UTC(),
		DurationSec:   durationSec,
		Profile:       data,
		Runtime:       collectRuntimeSnapshot(),
	}

	if err := a.send(ctx, client, url, up); err != nil {
		a.Logger.Warn("profile upload failed", "kind", kind, "error", err)
		return
	}
	a.Logger.Debug("profile uploaded", "kind", kind, "bytes", len(data))
}

// captureCPU runs a CPU profile for the configured duration.
func captureCPU(ctx context.Context, seconds int) ([]byte, error) {
	var buf bytes.Buffer
	if err := pprof.StartCPUProfile(&buf); err != nil {
		return nil, fmt.Errorf("start cpu profile: %w", err)
	}

	timer := time.NewTimer(time.Duration(seconds) * time.Second)
	defer timer.Stop()

	select {
	case <-ctx.Done():
	case <-timer.C:
	}

	pprof.StopCPUProfile()

	// A cancelled profile is a partial one, the aggregator would read
	// it as a full-length sample and understate CPU cost.
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return buf.Bytes(), nil
}

// captureNamed writes one of the runtime's named profiles in pprof's
// compressed protobuf form.
func captureNamed(name string) ([]byte, error) {
	p := pprof.Lookup(name)
	if p == nil {
		return nil, fmt.Errorf("no profile named %q", name)
	}

	if name == "heap" {
		runtime.GC()
	}

	var buf bytes.Buffer
	if err := p.WriteTo(&buf, 0); err != nil {
		return nil, fmt.Errorf("write %s profile: %w", name, err)
	}
	return buf.Bytes(), nil
}

func (a *Agent) send(ctx context.Context, client *http.Client, url string, up profileUpload) error {
	body, err := json.Marshal(up)
	if err != nil {
		return fmt.Errorf("marshal %s profile: %w", up.Kind, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", version.UserAgent("agent-profiler"))

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("post %s profile: %w", up.Kind, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("aggregator returned %s for %s profile", resp.Status, up.Kind)
	}
	return nil
}
