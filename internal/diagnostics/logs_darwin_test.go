//go:build darwin

package diagnostics

import (
	"context"
	"testing"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func TestFetchLogs_Integration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := protocol.LogRequest{
		MinLevel: protocol.LevelError,
	}

	logs, err := FetchLogs(ctx, req)
	if err != nil {
		t.Fatalf("FetchLogs failed: %v", err)
	}

	t.Logf("Successfully fetched %d logs from macOS", len(logs))

	collapsed := 0
	largest := 0

	for i, entry := range logs {
		if entry.Timestamp == 0 {
			t.Errorf("Log %d: Expected non-zero timestamp", i)
		}
		if entry.Message == "" {
			t.Errorf("Log %d: Expected non-empty message", i)
		}
		if entry.Source == "" {
			t.Errorf("Log %d: Expected non-empty source", i)
		}

		// A collapsed run carries the count and the start of the run; a
		// one-off carries neither, so the two fields move together.
		switch {
		case entry.Count > 1:
			collapsed++
			if entry.Count > largest {
				largest = entry.Count
			}
			if entry.FirstSeen == 0 {
				t.Errorf("Log %d: Count=%d but FirstSeen is unset", i, entry.Count)
			}
			if entry.FirstSeen > entry.Timestamp {
				t.Errorf("Log %d: FirstSeen (%d) is after the kept occurrence (%d)",
					i, entry.FirstSeen, entry.Timestamp)
			}
		case entry.Count == 1:
			t.Errorf("Log %d: Count=1 should be omitted, not stored", i)
		default:
			if entry.FirstSeen != 0 {
				t.Errorf("Log %d: FirstSeen=%d on an uncollapsed entry", i, entry.FirstSeen)
			}
		}

		priority := levelToPriority(entry.Level)
		reqPriority := levelToPriority(req.MinLevel)

		if priority > reqPriority {
			t.Errorf("Log %d: Got severity %s (%d) but requested minimum was %s (%d)",
				i, entry.Level, priority, req.MinLevel, reqPriority)
		}
	}

	t.Logf("collapsed runs: %d (largest %d occurrences)", collapsed, largest)
}
