package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

const (
	// spoolFormatVersion guards against reading a file written by a
	// different layout.
	spoolFormatVersion = 1

	spoolFileName = "metrics-spool.json"

	// spoolMaxAge bounds how stale a replayed envelope may be.
	spoolMaxAge = 24 * time.Hour

	// spoolMaxEnvelopes caps what a single shutdown writes.
	spoolMaxEnvelopes = 50_000
)

// errSpoolVersion means the file exists but was written by another format.
var errSpoolVersion = errors.New("unsupported spool format")

// spoolFile is the on-disk layout. What lands on disk is the same JSON the
// sender would have produced.
type spoolFile struct {
	Version   int               `json:"version"`
	WrittenAt time.Time         `json:"written_at"`
	Envelopes []json.RawMessage `json:"envelopes"`
}

// spoolEnvelope is an envelope with its payload left undecoded so the type
// field can be read before dispatching through protocol.UnmarshalMetric.
type spoolEnvelope struct {
	Type      string          `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Hostname  string          `json:"hostname"`
	Data      json.RawMessage `json:"data"`
}

// spoolPath puts the spool beside the identity file, keeping the directory
// the platform or the config chose.
func spoolPath(identityPath string) string {
	return filepath.Join(filepath.Dir(identityPath), spoolFileName)
}

// writeSpool persists a batch via temp-and-rename.
func writeSpool(path string, batch []protocol.Envelope, now time.Time) error {
	if len(batch) == 0 {
		return nil
	}
	if len(batch) > spoolMaxEnvelopes {
		batch = batch[len(batch)-spoolMaxEnvelopes:]
	}

	raw := make([]json.RawMessage, 0, len(batch))
	for _, e := range batch {
		b, err := json.Marshal(e)
		if err != nil {
			continue
		}
		raw = append(raw, b)
	}
	if len(raw) == 0 {
		return nil
	}

	body, err := json.Marshal(spoolFile{
		Version:   spoolFormatVersion,
		WrittenAt: now,
		Envelopes: raw,
	})
	if err != nil {
		return fmt.Errorf("marshal spool: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create spool dir: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".spool-*")
	if err != nil {
		return fmt.Errorf("create temp spool: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(body); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write temp spool: %w", err)
	}

	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("sync temp spool: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close temp spool: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename spool: %w", err)
	}

	return nil
}

// readSpool loads a spool, dropping envelopes older than maxAge. Skips
// envelopes that fail to decode rather than failing the load.
//
// Decoding the array validates every element, so a corrupt file fails here
// and is discarded whole.
func readSpool(path string, now time.Time, maxAge time.Duration) ([]protocol.Envelope, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var file spoolFile
	if err := json.Unmarshal(body, &file); err != nil {
		return nil, fmt.Errorf("decode spool: %w", err)
	}
	if file.Version != spoolFormatVersion {
		return nil, fmt.Errorf("%w: got %d, want %d", errSpoolVersion, file.Version, spoolFormatVersion)
	}

	cutoff := now.Add(-maxAge)
	out := make([]protocol.Envelope, 0, len(file.Envelopes))

	for _, rawEnv := range file.Envelopes {
		var se spoolEnvelope
		if err := json.Unmarshal(rawEnv, &se); err != nil {
			continue
		}
		if maxAge > 0 && se.Timestamp.Before(cutoff) {
			continue
		}

		metric, err := protocol.UnmarshalMetric(se.Type, se.Data)
		if err != nil {
			continue
		}

		out = append(out, protocol.Envelope{
			Type:      se.Type,
			Timestamp: se.Timestamp,
			Hostname:  se.Hostname,
			Data:      metric,
		})
	}

	return out, nil
}

// saveSpool drains the cache to disk. Called from Shutdown after the waitgroup
// has drained, ensuring nothing is adding to the cache concurrently.
func (a *Agent) saveSpool() {
	path := spoolPath(a.Config.IdentityPath)

	batch := a.cache.Drain()
	if len(batch) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			a.Logger.Warn("could not clear spool", "error", err)
		}
		return
	}

	if err := writeSpool(path, batch, time.Now()); err != nil {
		a.Logger.Warn("could not write spool", "error", err, "envelopes", len(batch))
		return
	}
	a.Logger.Info("spooled unsent metrics", "envelopes", len(batch), "path", path)
}

// loadSpool restores a previous run's backlog into the cache. The file is removed
// once read, so a crash loop replays it at most once.
func (a *Agent) loadSpool() {
	path := spoolPath(a.Config.IdentityPath)

	batch, err := readSpool(path, time.Now(), spoolMaxAge)
	if err != nil {
		if !os.IsNotExist(err) {
			a.Logger.Warn("could not read spool", "error", err, "path", path)
			if rmErr := os.Remove(path); rmErr != nil && !os.IsNotExist(rmErr) {
				a.Logger.Warn("could not remove unreadable spool", "error", rmErr)
			}
		}
		return
	}

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		a.Logger.Warn("could not remove spool after load", "error", err)
	}

	if len(batch) == 0 {
		return
	}

	a.cache.Add(batch)
	a.Logger.Info("restored spooled metrics", "envelopes", len(batch))
}
