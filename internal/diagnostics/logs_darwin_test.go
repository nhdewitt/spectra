//go:build darwin

package diagnostics

import (
	"context"
	"strings"
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

	dedupCount := 0

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
		if strings.Contains(entry.Message, "(further duplicates suppressed)") {
			dedupCount++
		}

		priority := levelToPriority(entry.Level)
		reqPriority := levelToPriority(req.MinLevel)

		if priority > reqPriority {
			t.Errorf("Log %d: Got severity %s (%d) but requested minimum was %s (%d)",
				i, entry.Level, priority, req.MinLevel, reqPriority)
		}
	}

	t.Logf("dedupCount: %d", dedupCount)
}
