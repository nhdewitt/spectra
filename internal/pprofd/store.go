package pprofd

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

// Upload mirrors what the agent's profiler sends. The profile bytes are stored
// beside the metadata rather than inside it, so go tool pprof can fetch the raw
// file without the caller decoding JSON first.
type Upload struct {
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
	CGO          bool   `json:"cgo"`

	Kind        string    `json:"kind"`
	CapturedAt  time.Time `json:"captured_at"`
	DurationSec int       `json:"duration_sec"`
	Profile     []byte    `json:"profile"`

	Runtime RuntimeSnapshot `json:"runtime"`
}

// runtimeSnapshot is the scalar state a pprof profile does not carry.
type RuntimeSnapshot struct {
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

// Record is one stored profile without its bytes.
type Record struct {
	ID string `json:"id"`
	Upload
}

// MarshalJSON drops the profile bytes from listings.
func (r Record) MarshalJSON() ([]byte, error) {
	type alias Record
	clone := alias(r)
	clone.Profile = nil
	return json.Marshal(clone)
}

// Store keeps profiles as files on disk. One .pprof and one .json per
// upload under a per-agent directory.
type Store struct {
	root   string
	maxAge time.Duration
}

// Mirrors internal/agent's applyMemoryLimit so a summary can say whether a
// host's ceiling was derived or clamped.
const (
	memLimitDivisor       = 4
	minMemLimit     int64 = 48 << 20
	maxMemLimit     int64 = 2 << 30
)

var safeID = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

func validSegment(v string) bool {
	return safeID.MatchString(v) && v != "." && v != ".."
}

// ErrNotFound is returned for an unknown agent or profile ID.
var ErrNotFound = errors.New("not found")

func NewStore(root string, maxAge time.Duration) (*Store, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("create store root: %w", err)
	}
	return &Store{root: root, maxAge: maxAge}, nil
}

// agentDir rejects anything that could climb out of the root.
// Agent IDs are UUIDs, but they arrive in a request body.
func (s *Store) agentDir(agentID string) (string, error) {
	if !validSegment(agentID) {
		return "", fmt.Errorf("invalid agent id %q", agentID)
	}

	dir := filepath.Join(s.root, agentID)

	root, err := filepath.Abs(s.root)
	if err != nil {
		return "", fmt.Errorf("resolve store root: %w", err)
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolve agent dir: %w", err)
	}
	if abs != root && !strings.HasPrefix(abs, root+string(filepath.Separator)) {
		return "", fmt.Errorf("agent id %q escapes the store root", agentID)
	}

	return dir, nil
}

// Put writes one upload. The ID encodes capture time first, so a
// lexical sort of the directory is a chronological one.
func (s *Store) Put(up Upload) (string, error) {
	if up.AgentID == "" {
		return "", errors.New("upload has no agent id")
	}
	if up.Kind == "" {
		return "", errors.New("upload has no kind")
	}
	if len(up.Profile) == 0 {
		return "", errors.New("upload has no profile bytes")
	}
	if !validSegment(up.Kind) {
		return "", fmt.Errorf("invalid kind %q", up.Kind)
	}

	dir, err := s.agentDir(up.AgentID)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create agent dir: %w", err)
	}

	if up.CapturedAt.IsZero() {
		up.CapturedAt = time.Now().UTC()
	}
	id := fmt.Sprintf("%s-%s", up.CapturedAt.UTC().Format("20060102T150405.000"), up.Kind)

	if err := writeAtomic(filepath.Join(dir, id+".pprof"), up.Profile); err != nil {
		return "", err
	}

	meta := Record{ID: id, Upload: up}
	body, err := json.Marshal(meta)
	if err != nil {
		return "", fmt.Errorf("marshal metadata: %w", err)
	}
	if err := writeAtomic(filepath.Join(dir, id+".json"), body); err != nil {
		return "", err
	}

	return id, nil
}

// writeAtomic avoids a reader seeing a half-written profile, since uploads and downloads are concurrent.
func writeAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename into place: %w", err)
	}
	return nil
}

// Agents lists agent IDs that have at least one stored profile.
func (s *Store) Agents() ([]string, error) {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return nil, fmt.Errorf("read store root: %w", err)
	}

	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	slices.Sort(out)
	return out, nil
}

// List returns an agent's records, newest first.
func (s *Store) List(agentID string) ([]Record, error) {
	dir, err := s.agentDir(agentID)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("read agent dir: %w", err)
	}

	var out []Record
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		body, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var rec Record
		if err := json.Unmarshal(body, &rec); err != nil {
			continue
		}
		rec.Profile = nil
		out = append(out, rec)
	}

	slices.SortFunc(out, func(a, b Record) int { return cmp.Compare(b.ID, a.ID) })
	return out, nil
}

// Profile returns the raw pprof bytes for one record.
func (s *Store) Profile(agentID, id string) ([]byte, error) {
	dir, err := s.agentDir(agentID)
	if err != nil {
		return nil, err
	}
	if !validSegment(id) {
		return nil, fmt.Errorf("invalid profile id %q", id)
	}

	data, err := os.ReadFile(filepath.Join(dir, id+".pprof"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("read profile: %w", err)
	}
	return data, nil
}

// Summary is one agent's latest state plus the rates derived from the sample before it.
type Summary struct {
	AgentID  string `json:"agent_id"`
	Hostname string `json:"hostname"`

	OS       string `json:"os"`
	Arch     string `json:"arch"`
	CPUModel string `json:"cpu_model"`
	CPUCores int    `json:"cpu_cores"`

	AgentVersion string `json:"agent_version"`
	GoVersion    string `json:"go_version"`
	CGO          bool   `json:"cgo"`

	LastSeen time.Time `json:"last_seen"`
	Profiles int       `json:"profiles"`

	HeapLiveBytes uint64 `json:"heap_live_bytes"`
	GOMEMLIMIT    int64  `json:"gomemlimit"`
	RAMTotalBytes uint64 `json:"ram_total_bytes"`
	Goroutines    uint64 `json:"goroutines"`

	// LimitClamped is true when the ceiling came from minMemLimit or
	// maxMemLimit rather than from total/memLimitDivisor. A clamped host tells
	// you nothing about whether the divisor is right.
	LimitClamped bool `json:"limit_clamped"`

	// LimitPctOfRAM is the ceiling as a fraction of host RAM. Comparable
	// across hosts in a way the raw byte count is not.
	LimitPctOfRAM float64 `json:"limit_pct_of_ram"`

	// HeapHeadroom is heap live as a fraction of the host's own ceiling.
	// applyMemoryLimit derives that ceiling from host RAM, so the raw byte
	// count says nothing about how close an agent is to GC pressure.
	HeapHeadroom float64 `json:"heap_headroom"`

	// GCCPUFraction is GC CPU seconds per wall second between the two most
	// recent captures. Nil when there is only one sample to work from.
	GCCPUFraction *float64 `json:"gc_cpu_fraction,omitempty"`

	// GCRate is GC cycles per second over the same window.
	GCRate *float64 `json:"gc_rate,omitempty"`
}

// Summaries returns one Summary per agent, sorted by hostname.
func (s *Store) Summaries() ([]Summary, error) {
	agents, err := s.Agents()
	if err != nil {
		return nil, err
	}

	out := make([]Summary, 0, len(agents))
	for _, agent := range agents {
		recs, err := s.List(agent)
		if err != nil || len(recs) == 0 {
			continue
		}
		out = append(out, summarize(agent, recs))
	}

	slices.SortFunc(out, func(a, b Summary) int { return cmp.Compare(a.Hostname, b.Hostname) })
	return out, nil
}

const minRateWindow = 30 * time.Second

// priorCycle finds the most recent record far enough before head to divide by.
// rest must be newest-first and most not include head.
func priorCycle(head Record, rest []Record) (Record, bool) {
	for _, rec := range rest {
		if head.CapturedAt.Sub(rec.CapturedAt) >= minRateWindow {
			return rec, true
		}
	}
	return Record{}, false
}

func summarize(agentID string, recs []Record) Summary {
	head := recs[0]

	sum := Summary{
		AgentID:       agentID,
		Hostname:      head.Hostname,
		OS:            head.OS,
		Arch:          head.Arch,
		CPUModel:      head.CPUModel,
		CPUCores:      head.CPUCores,
		AgentVersion:  head.AgentVersion,
		GoVersion:     head.GoVersion,
		CGO:           head.CGO,
		LastSeen:      head.CapturedAt,
		Profiles:      len(recs),
		HeapLiveBytes: head.Runtime.HeapLiveBytes,
		GOMEMLIMIT:    head.Runtime.GOMEMLIMIT,
		RAMTotalBytes: head.RAMTotalBytes,
		Goroutines:    head.Runtime.Goroutines,
	}

	if head.Runtime.GOMEMLIMIT > 0 {
		sum.HeapHeadroom = float64(head.Runtime.HeapLiveBytes) / float64(head.Runtime.GOMEMLIMIT)
	}

	if head.RAMTotalBytes > 0 && head.Runtime.GOMEMLIMIT > 0 {
		sum.LimitPctOfRAM = float64(head.Runtime.GOMEMLIMIT) / float64(head.RAMTotalBytes)

		derived := int64(head.RAMTotalBytes / memLimitDivisor)
		sum.LimitClamped = derived < minMemLimit || derived > maxMemLimit
	}

	prev, ok := priorCycle(head, recs[1:])
	if !ok {
		return sum
	}

	wall := head.CapturedAt.Sub(prev.CapturedAt).Seconds()

	if head.Runtime.GCCPUSecs >= prev.Runtime.GCCPUSecs {
		frac := (head.Runtime.GCCPUSecs - prev.Runtime.GCCPUSecs) / wall
		sum.GCCPUFraction = &frac
	}
	if head.Runtime.GCCycles >= prev.Runtime.GCCycles {
		rate := float64(head.Runtime.GCCycles-prev.Runtime.GCCycles) / wall
		sum.GCRate = &rate
	}

	return sum
}

// Prune deletes profiles older than maxAge and returns how many it removed.
// A zero maxAge disables it.
func (s *Store) Prune(now time.Time) (int, error) {
	if s.maxAge <= 0 {
		return 0, nil
	}

	agents, err := s.Agents()
	if err != nil {
		return 0, err
	}

	cutoff := now.Add(-s.maxAge)
	removed := 0

	for _, agent := range agents {
		recs, err := s.List(agent)
		if err != nil {
			continue
		}
		dir, err := s.agentDir(agent)
		if err != nil {
			continue
		}
		for _, rec := range recs {
			if rec.CapturedAt.After(cutoff) {
				continue
			}
			os.Remove(filepath.Join(dir, rec.ID+".json"))
			os.Remove(filepath.Join(dir, rec.ID+".pprof"))
			removed++
		}
	}

	return removed, nil
}
