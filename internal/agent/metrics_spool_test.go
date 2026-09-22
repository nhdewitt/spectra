package agent

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func spoolEnvelopeAt(metricType string, ts time.Time) protocol.Envelope {
	return protocol.Envelope{
		Type:      metricType,
		Timestamp: ts,
		Hostname:  "test-host",
		Data:      &protocol.CPUMetric{Usage: 42.0},
	}
}

func tempSpool(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), spoolFileName)
}

func TestSpool_RoundTrip(t *testing.T) {
	path := tempSpool(t)
	now := time.Now().UTC().Truncate(time.Second)

	batch := []protocol.Envelope{
		spoolEnvelopeAt("cpu", now.Add(-time.Minute)),
		spoolEnvelopeAt("cpu", now),
	}

	if err := writeSpool(path, batch, now); err != nil {
		t.Fatalf("writeSpool: %v", err)
	}

	back, err := readSpool(path, now, spoolMaxAge)
	if err != nil {
		t.Fatalf("readSpool: %v", err)
	}
	if len(back) != 2 {
		t.Fatalf("got %d envelopes, want 2", len(back))
	}

	for i, e := range back {
		if e.Type != "cpu" || e.Hostname != "test-host" {
			t.Errorf("envelope %d = %+v", i, e)
		}
		cpu, ok := e.Data.(*protocol.CPUMetric)
		if !ok {
			t.Fatalf("envelope %d data is %T, want *protocol.CPUMetric", i, e.Data)
		}
		if cpu.Usage != 42.0 {
			t.Errorf("envelope %d usage = %v, want 42", i, cpu.Usage)
		}
	}

	if !back[1].Timestamp.Equal(now) {
		t.Errorf("timestamp = %v, want %v", back[1].Timestamp, now)
	}
}

// An agent that was off for a week should not replay a week of backlog.
func TestSpool_DropsEnvelopesPastTheCutoff(t *testing.T) {
	path := tempSpool(t)
	now := time.Now().UTC()

	batch := []protocol.Envelope{
		spoolEnvelopeAt("cpu", now.Add(-48*time.Hour)),
		spoolEnvelopeAt("cpu", now.Add(-30*time.Hour)),
		spoolEnvelopeAt("cpu", now.Add(-time.Hour)),
	}

	if err := writeSpool(path, batch, now); err != nil {
		t.Fatalf("writeSpool: %v", err)
	}

	back, err := readSpool(path, now, spoolMaxAge)
	if err != nil {
		t.Fatalf("readSpool: %v", err)
	}
	if len(back) != 1 {
		t.Fatalf("got %d envelopes, want 1", len(back))
	}
}

// The cutoff is per envelope, not per file: a spool written seconds ago can
// still hold envelopes from well before it.
func TestSpool_CutoffIsPerEnvelopeNotPerFile(t *testing.T) {
	path := tempSpool(t)
	now := time.Now().UTC()

	batch := []protocol.Envelope{spoolEnvelopeAt("cpu", now.Add(-72*time.Hour))}

	if err := writeSpool(path, batch, now); err != nil {
		t.Fatalf("writeSpool: %v", err)
	}

	back, err := readSpool(path, now, spoolMaxAge)
	if err != nil {
		t.Fatalf("readSpool: %v", err)
	}
	if len(back) != 0 {
		t.Errorf("got %d envelopes from a fresh file of stale entries, want 0", len(back))
	}
}

func TestSpool_ZeroMaxAgeKeepsEverything(t *testing.T) {
	path := tempSpool(t)
	now := time.Now().UTC()

	batch := []protocol.Envelope{spoolEnvelopeAt("cpu", now.Add(-1000*time.Hour))}

	if err := writeSpool(path, batch, now); err != nil {
		t.Fatalf("writeSpool: %v", err)
	}

	back, err := readSpool(path, now, 0)
	if err != nil {
		t.Fatalf("readSpool: %v", err)
	}
	if len(back) != 1 {
		t.Errorf("got %d envelopes, want 1", len(back))
	}
}

// A partial decode would replay wrong metrics, so a version mismatch discards
// the file rather than guessing.
func TestSpool_RejectsOtherFormatVersions(t *testing.T) {
	path := tempSpool(t)

	body, err := json.Marshal(spoolFile{
		Version:   spoolFormatVersion + 1,
		WrittenAt: time.Now(),
		Envelopes: []json.RawMessage{json.RawMessage(`{"type":"cpu"}`)},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err = readSpool(path, time.Now(), spoolMaxAge)
	if !errors.Is(err, errSpoolVersion) {
		t.Errorf("error = %v, want errSpoolVersion", err)
	}
}

// A metric type this build does not know -- an older agent reading a spool
// written by a newer one -- should not cost the rest of the backlog.
func TestSpool_SkipsUnknownMetricTypes(t *testing.T) {
	path := tempSpool(t)
	now := time.Now().UTC()

	good, err := json.Marshal(spoolEnvelopeAt("cpu", now))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	body, err := json.Marshal(spoolFile{
		Version:   spoolFormatVersion,
		WrittenAt: now,
		Envelopes: []json.RawMessage{
			json.RawMessage(`{"type":"not_a_metric","timestamp":"` + now.Format(time.RFC3339) + `","data":{}}`),
			json.RawMessage(good),
		},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	back, err := readSpool(path, now, spoolMaxAge)
	if err != nil {
		t.Fatalf("readSpool: %v", err)
	}
	if len(back) != 1 {
		t.Fatalf("got %d envelopes, want just the known one", len(back))
	}
	if back[0].Type != "cpu" {
		t.Errorf("kept envelope type = %q, want cpu", back[0].Type)
	}
}

// Malformed JSON anywhere in the array fails the outer decode, so a truncated
// or corrupted spool is discarded whole rather than partially replayed.
func TestSpool_CorruptFileIsRejectedWhole(t *testing.T) {
	path := tempSpool(t)

	if err := os.WriteFile(path, []byte(`{"version":1,"envelopes":[{ broken`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := readSpool(path, time.Now(), spoolMaxAge); err == nil {
		t.Error("expected an error for a corrupt spool")
	}
}

func TestSpool_MissingFile(t *testing.T) {
	_, err := readSpool(tempSpool(t), time.Now(), spoolMaxAge)
	if !os.IsNotExist(err) {
		t.Errorf("error = %v, want a not-exist error", err)
	}
}

func TestSpool_EmptyBatchWritesNothing(t *testing.T) {
	path := tempSpool(t)

	if err := writeSpool(path, nil, time.Now()); err != nil {
		t.Fatalf("writeSpool: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("an empty batch created a spool file")
	}
}

// A half-written spool read at the next start would lose the whole backlog,
// so the file is only ever published by the rename.
func TestSpool_LeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, spoolFileName)
	now := time.Now().UTC()

	if err := writeSpool(path, []protocol.Envelope{spoolEnvelopeAt("cpu", now)}, now); err != nil {
		t.Fatalf("writeSpool: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".spool-") {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
	if len(entries) != 1 {
		t.Errorf("got %d files, want 1", len(entries))
	}
}

func TestSpool_CapsEnvelopeCount(t *testing.T) {
	path := tempSpool(t)
	now := time.Now().UTC()

	batch := make([]protocol.Envelope, spoolMaxEnvelopes+10)
	for i := range batch {
		batch[i] = spoolEnvelopeAt("cpu", now)
	}

	if err := writeSpool(path, batch, now); err != nil {
		t.Fatalf("writeSpool: %v", err)
	}

	back, err := readSpool(path, now, spoolMaxAge)
	if err != nil {
		t.Fatalf("readSpool: %v", err)
	}
	if len(back) != spoolMaxEnvelopes {
		t.Errorf("got %d envelopes, want %d", len(back), spoolMaxEnvelopes)
	}
}

func TestSpoolPath_SitsBesideIdentity(t *testing.T) {
	got := spoolPath(filepath.Join("/etc", "spectra", "agent-id.json"))
	want := filepath.Join("/etc", "spectra", spoolFileName)
	if got != want {
		t.Errorf("spoolPath = %q, want %q", got, want)
	}
}

// --- Agent hooks ---

func TestAgent_SaveAndLoadSpool(t *testing.T) {
	a := newTestAgentWithLogger()
	a.Config.IdentityPath = filepath.Join(t.TempDir(), "agent-id.json")

	a.cache.Add([]protocol.Envelope{
		spoolEnvelopeAt("cpu", time.Now()),
		spoolEnvelopeAt("cpu", time.Now()),
	})

	a.saveSpool()

	if a.cache.Len() != 0 {
		t.Errorf("cache holds %d after saveSpool, want 0", a.cache.Len())
	}
	if _, err := os.Stat(spoolPath(a.Config.IdentityPath)); err != nil {
		t.Fatalf("spool not written: %v", err)
	}

	a.loadSpool()

	if a.cache.Len() != 2 {
		t.Errorf("cache holds %d after loadSpool, want 2", a.cache.Len())
	}
}

// A crash loop must not replay the same backlog forever.
func TestAgent_LoadSpoolRemovesTheFile(t *testing.T) {
	a := newTestAgentWithLogger()
	a.Config.IdentityPath = filepath.Join(t.TempDir(), "agent-id.json")
	path := spoolPath(a.Config.IdentityPath)

	a.cache.Add([]protocol.Envelope{spoolEnvelopeAt("cpu", time.Now())})
	a.saveSpool()
	a.loadSpool()

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("spool survived the load")
	}

	a.cache.Drain()
	a.loadSpool()

	if a.cache.Len() != 0 {
		t.Errorf("cache holds %d after a second load, want 0", a.cache.Len())
	}
}

// An empty cache clears any spool from a previous run, so a restart with
// nothing pending does not replay the last one.
func TestAgent_SaveSpoolClearsStaleFile(t *testing.T) {
	a := newTestAgentWithLogger()
	a.Config.IdentityPath = filepath.Join(t.TempDir(), "agent-id.json")
	path := spoolPath(a.Config.IdentityPath)

	a.cache.Add([]protocol.Envelope{spoolEnvelopeAt("cpu", time.Now())})
	a.saveSpool()

	a.saveSpool()

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("an empty cache left the previous spool in place")
	}
}
