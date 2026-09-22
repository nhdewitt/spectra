package pprofd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testUpload(agentID, kind string, at time.Time) Upload {
	return Upload{
		AgentID:      agentID,
		Hostname:     "test-host",
		OS:           "linux",
		Arch:         "arm64",
		CPUModel:     "Test CPU",
		CPUCores:     4,
		AgentVersion: "0.1.0",
		AgentCommit:  "abc1234",
		GoVersion:    "go1.26.2",
		Kind:         kind,
		CapturedAt:   at,
		DurationSec:  30,
		Profile:      []byte{0x1f, 0x8b, 0x01, 0x02},
		Runtime: RuntimeSnapshot{
			GOMAXPROCS: 4,
			GOMEMLIMIT: 512 << 20,
			Goroutines: 17,
		},
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := NewStore(t.TempDir(), 0)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return s
}

func TestStore_PutAndFetch(t *testing.T) {
	s := newTestStore(t)
	at := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)

	id, err := s.Put(testUpload("agent-1", "cpu", at))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	data, err := s.Profile("agent-1", id)
	if err != nil {
		t.Fatalf("Profile: %v", err)
	}
	if len(data) != 4 {
		t.Errorf("profile bytes = %d, want 4", len(data))
	}

	recs, err := s.List("agent-1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("got %d records, want 1", len(recs))
	}
	if recs[0].Runtime.Goroutines != 17 {
		t.Errorf("runtime snapshot lost: %+v", recs[0].Runtime)
	}
	if recs[0].CPUModel != "Test CPU" || recs[0].CPUCores != 4 {
		t.Errorf("host facts lost: %+v", recs[0])
	}
}

// The ID leads with the capture timestamp so a lexical sort is chronological.
func TestStore_ListIsNewestFirst(t *testing.T) {
	s := newTestStore(t)
	base := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)

	for i := range 3 {
		if _, err := s.Put(testUpload("agent-1", "cpu", base.Add(time.Duration(i)*time.Minute))); err != nil {
			t.Fatalf("Put: %v", err)
		}
	}

	recs, err := s.List("agent-1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(recs) != 3 {
		t.Fatalf("got %d records, want 3", len(recs))
	}
	for i := 1; i < len(recs); i++ {
		if !recs[i-1].CapturedAt.After(recs[i].CapturedAt) {
			t.Errorf("record %d is not newer than %d", i-1, i)
		}
	}
}

// Listings would otherwise carry every profile base64-encoded.
func TestRecord_MarshalDropsProfileBytes(t *testing.T) {
	rec := Record{ID: "x", Upload: testUpload("agent-1", "heap", time.Now())}

	body, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var back map[string]any
	if err := json.Unmarshal(body, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if v, ok := back["profile"]; ok && v != nil {
		t.Errorf("profile bytes survived marshalling: %v", v)
	}
	if back["hostname"] != "test-host" {
		t.Error("metadata was dropped along with the bytes")
	}
}

// Agent IDs arrive in a request body, so they must not be able to climb out of
// the store root.
func TestStore_RejectsTraversal(t *testing.T) {
	s := newTestStore(t)

	for _, bad := range []string{"../etc", "a/b", "..", "", "with space"} {
		if _, err := s.Put(testUpload(bad, "cpu", time.Now())); err == nil {
			t.Errorf("Put accepted agent id %q", bad)
		}
		if _, err := s.List(bad); err == nil {
			t.Errorf("List accepted agent id %q", bad)
		}
	}
}

func TestStore_RejectsBadKind(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.Put(testUpload("agent-1", "../escape", time.Now())); err == nil {
		t.Error("Put accepted a traversing kind")
	}
}

func TestStore_RejectsEmptyProfile(t *testing.T) {
	s := newTestStore(t)

	up := testUpload("agent-1", "cpu", time.Now())
	up.Profile = nil

	if _, err := s.Put(up); err == nil {
		t.Error("Put accepted an upload with no profile bytes")
	}
}

func TestStore_UnknownAgentAndProfile(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.List("agent-404"); err != ErrNotFound {
		t.Errorf("List error = %v, want ErrNotFound", err)
	}

	if _, err := s.Put(testUpload("agent-1", "cpu", time.Now())); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if _, err := s.Profile("agent-1", "20260101T000000.000-cpu"); err != ErrNotFound {
		t.Errorf("Profile error = %v, want ErrNotFound", err)
	}
}

func TestStore_Agents(t *testing.T) {
	s := newTestStore(t)

	for _, id := range []string{"agent-b", "agent-a"} {
		if _, err := s.Put(testUpload(id, "cpu", time.Now())); err != nil {
			t.Fatalf("Put: %v", err)
		}
	}

	agents, err := s.Agents()
	if err != nil {
		t.Fatalf("Agents: %v", err)
	}
	if len(agents) != 2 || agents[0] != "agent-a" || agents[1] != "agent-b" {
		t.Errorf("agents = %v, want sorted [agent-a agent-b]", agents)
	}
}

func TestStore_Prune(t *testing.T) {
	s, err := NewStore(t.TempDir(), time.Hour)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	old := now.Add(-2 * time.Hour)

	if _, err := s.Put(testUpload("agent-1", "cpu", old)); err != nil {
		t.Fatalf("Put: %v", err)
	}
	keepID, err := s.Put(testUpload("agent-1", "heap", now.Add(-time.Minute)))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	n, err := s.Prune(now)
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if n != 1 {
		t.Errorf("pruned %d, want 1", n)
	}

	recs, err := s.List("agent-1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(recs) != 1 || recs[0].ID != keepID {
		t.Errorf("survivors = %v, want just %s", recs, keepID)
	}
}

func TestStore_PruneDisabled(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.Put(testUpload("agent-1", "cpu", time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))); err != nil {
		t.Fatalf("Put: %v", err)
	}

	n, err := s.Prune(time.Now())
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if n != 0 {
		t.Errorf("pruned %d with retention disabled, want 0", n)
	}
}

// A half-written profile must never be readable, since uploads and downloads
// are concurrent.
func TestStore_LeavesNoTempFiles(t *testing.T) {
	root := t.TempDir()
	s, err := NewStore(root, 0)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	if _, err := s.Put(testUpload("agent-1", "cpu", time.Now())); err != nil {
		t.Fatalf("Put: %v", err)
	}

	entries, err := os.ReadDir(filepath.Join(root, "agent-1"))
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if len(e.Name()) > 0 && e.Name()[0] == '.' {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
	if len(entries) != 2 {
		t.Errorf("got %d files, want 2 (.pprof and .json)", len(entries))
	}
}

// --- Summaries ---
// Cumulative counters are not comparable across hosts that started at
// different times, so the derived rates are what the comparison view uses.

func TestSummarize_DerivesRatesFromTwoCycles(t *testing.T) {
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)

	newer := Record{ID: "b", Upload: testUpload("agent-1", "heap", now)}
	newer.Runtime.GCCPUSecs = 12
	newer.Runtime.GCCycles = 140

	older := Record{ID: "a", Upload: testUpload("agent-1", "cpu", now.Add(-100*time.Second))}
	older.Runtime.GCCPUSecs = 10
	older.Runtime.GCCycles = 100

	sum := summarize("agent-1", []Record{newer, older})

	if sum.GCCPUFraction == nil {
		t.Fatal("GC CPU fraction was not derived")
	}
	if got := *sum.GCCPUFraction; got < 0.019 || got > 0.021 {
		t.Errorf("GC CPU fraction = %v, want ~0.02", got)
	}
	if sum.GCRate == nil {
		t.Fatal("GC rate was not derived")
	}
	if got := *sum.GCRate; got < 0.39 || got > 0.41 {
		t.Errorf("GC rate = %v, want ~0.4", got)
	}
}

// All three profiles in a round land within milliseconds of each other. Using
// one as the reference for the next divided a one-cycle delta by a sub-second
// gap and reported hundreds of collections per second.
func TestSummarize_IgnoresSameCycleCaptures(t *testing.T) {
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)

	recs := []Record{
		{ID: "c", Upload: testUpload("agent-1", "goroutine", now)},
		{ID: "b", Upload: testUpload("agent-1", "heap", now.Add(-2*time.Millisecond))},
		{ID: "a", Upload: testUpload("agent-1", "cpu", now.Add(-31*time.Millisecond))},
	}
	recs[0].Runtime.GCCycles = 5
	recs[1].Runtime.GCCycles = 4
	recs[2].Runtime.GCCycles = 3

	sum := summarize("agent-1", recs)

	if sum.GCRate != nil {
		t.Errorf("GC rate = %v from one cycle, want unset", *sum.GCRate)
	}
	if sum.GCCPUFraction != nil {
		t.Errorf("GC CPU fraction = %v from one cycle, want unset", *sum.GCCPUFraction)
	}
}

// The reference is the newest record outside the current round, not simply the
// second one in the list.
func TestSummarize_SkipsPastSameCycleToPriorRound(t *testing.T) {
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)

	recs := []Record{
		{ID: "d", Upload: testUpload("agent-1", "goroutine", now)},
		{ID: "c", Upload: testUpload("agent-1", "heap", now.Add(-3*time.Millisecond))},
		{ID: "b", Upload: testUpload("agent-1", "cpu", now.Add(-31*time.Millisecond))},
		{ID: "a", Upload: testUpload("agent-1", "cpu", now.Add(-15*time.Minute))},
	}
	recs[0].Runtime.GCCycles = 190
	recs[1].Runtime.GCCycles = 189
	recs[2].Runtime.GCCycles = 188
	recs[3].Runtime.GCCycles = 100

	sum := summarize("agent-1", recs)

	if sum.GCRate == nil {
		t.Fatal("no rate derived against the prior round")
	}
	// 90 cycles over 900 seconds.
	if got := *sum.GCRate; got < 0.099 || got > 0.101 {
		t.Errorf("GC rate = %v, want ~0.1", got)
	}
}

func TestSummarize_SingleCaptureHasNoRates(t *testing.T) {
	rec := Record{ID: "a", Upload: testUpload("agent-1", "cpu", time.Now())}

	sum := summarize("agent-1", []Record{rec})

	if sum.GCCPUFraction != nil || sum.GCRate != nil {
		t.Error("rates were derived from a single capture")
	}
	if sum.Profiles != 1 {
		t.Errorf("profiles = %d, want 1", sum.Profiles)
	}
}

// A restarted agent resets its counters; a negative rate is worse than none.
func TestSummarize_IgnoresCounterReset(t *testing.T) {
	now := time.Now()

	newer := Record{ID: "b", Upload: testUpload("agent-1", "heap", now)}
	newer.Runtime.GCCPUSecs = 1
	newer.Runtime.GCCycles = 5

	older := Record{ID: "a", Upload: testUpload("agent-1", "cpu", now.Add(-time.Minute))}
	older.Runtime.GCCPUSecs = 900
	older.Runtime.GCCycles = 9000

	sum := summarize("agent-1", []Record{newer, older})

	if sum.GCCPUFraction != nil {
		t.Errorf("GC CPU fraction = %v after a reset, want unset", *sum.GCCPUFraction)
	}
	if sum.GCRate != nil {
		t.Errorf("GC rate = %v after a reset, want unset", *sum.GCRate)
	}
}

// Raw heap bytes say nothing about GC pressure when every host has a ceiling
// derived from its own RAM.
func TestSummarize_HeapHeadroomIsRelativeToLimit(t *testing.T) {
	rec := Record{ID: "a", Upload: testUpload("agent-1", "heap", time.Now())}
	rec.Runtime.HeapLiveBytes = 128 << 20
	rec.Runtime.GOMEMLIMIT = 512 << 20

	sum := summarize("agent-1", []Record{rec})

	if got := sum.HeapHeadroom; got < 0.24 || got > 0.26 {
		t.Errorf("heap headroom = %v, want ~0.25", got)
	}
}

func TestSummarize_NoLimitLeavesHeadroomZero(t *testing.T) {
	rec := Record{ID: "a", Upload: testUpload("agent-1", "heap", time.Now())}
	rec.Runtime.HeapLiveBytes = 128 << 20
	rec.Runtime.GOMEMLIMIT = 0

	if got := summarize("agent-1", []Record{rec}).HeapHeadroom; got != 0 {
		t.Errorf("heap headroom = %v with no limit, want 0", got)
	}
}

func TestStore_SummariesSortedByHostname(t *testing.T) {
	s := newTestStore(t)

	for _, host := range []string{"zeta", "alpha"} {
		up := testUpload("agent-"+host, "cpu", time.Now())
		up.Hostname = host
		if _, err := s.Put(up); err != nil {
			t.Fatalf("Put: %v", err)
		}
	}

	sums, err := s.Summaries()
	if err != nil {
		t.Fatalf("Summaries: %v", err)
	}
	if len(sums) != 2 {
		t.Fatalf("got %d summaries, want 2", len(sums))
	}
	if sums[0].Hostname != "alpha" || sums[1].Hostname != "zeta" {
		t.Errorf("summaries = %s, %s; want alpha, zeta", sums[0].Hostname, sums[1].Hostname)
	}
}

// A host above 8GB clamps at maxMemLimit, so its ceiling says nothing about
// whether memLimitDivisor is right.
func TestSummarize_FlagsClampedMemoryLimit(t *testing.T) {
	rec := Record{ID: "a", Upload: testUpload("agent-1", "heap", time.Now())}
	rec.RAMTotalBytes = 16 << 30
	rec.Runtime.GOMEMLIMIT = maxMemLimit

	sum := summarize("agent-1", []Record{rec})

	if !sum.LimitClamped {
		t.Error("a 16GB host should be flagged as clamped")
	}
}

func TestSummarize_DerivedLimitIsNotClamped(t *testing.T) {
	rec := Record{ID: "a", Upload: testUpload("agent-1", "heap", time.Now())}
	rec.RAMTotalBytes = 2 << 30
	rec.Runtime.GOMEMLIMIT = (2 << 30) / memLimitDivisor

	sum := summarize("agent-1", []Record{rec})

	if sum.LimitClamped {
		t.Error("a 2GB host is inside the clamp range")
	}
	if got := sum.LimitPctOfRAM; got < 0.24 || got > 0.26 {
		t.Errorf("limit as fraction of RAM = %v, want ~0.25", got)
	}
}

// A very small host clamps at the bottom instead.
func TestSummarize_FlagsMinClamp(t *testing.T) {
	rec := Record{ID: "a", Upload: testUpload("agent-1", "heap", time.Now())}
	rec.RAMTotalBytes = 128 << 20
	rec.Runtime.GOMEMLIMIT = minMemLimit

	if !summarize("agent-1", []Record{rec}).LimitClamped {
		t.Error("a 128MB host should be flagged as clamped")
	}
}

func TestSummarize_NoRAMLeavesLimitFieldsZero(t *testing.T) {
	rec := Record{ID: "a", Upload: testUpload("agent-1", "heap", time.Now())}
	rec.RAMTotalBytes = 0
	rec.Runtime.GOMEMLIMIT = 512 << 20

	sum := summarize("agent-1", []Record{rec})

	if sum.LimitClamped || sum.LimitPctOfRAM != 0 {
		t.Errorf("limit fields derived without host RAM: %+v", sum)
	}
}
