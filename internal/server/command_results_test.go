package server

import (
	"testing"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

// expireNow runs the same expiry rule cleanup uses, without waiting on its
// one-minute ticker.
func expireNow(s *commandResultStore) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for id, entry := range s.entries {
		var expired bool
		if entry.Done {
			expired = entry.completedAt.Add(s.ttl).Before(now)
		} else {
			expired = entry.QueuedAt.Add(maxCommandLifetime).Before(now)
		}
		if expired {
			delete(s.entries, id)
		}
	}
}

// TestCommandResultStore_KeepsResultOfASlowCommand is the regression test.
// Retention used to run from QueuedAt for every entry, so a command that took
// nearly the whole TTL had its result deleted moments after it arrived -- and a
// command that ran to its deadline reported into an entry that was already
// gone, losing the timeout report entirely.
func TestCommandResultStore_KeepsResultOfASlowCommand(t *testing.T) {
	s := newCommandResultStore(10 * time.Minute)
	defer s.Stop()

	s.Track("cmd-slow", protocol.CmdUpdateAgent, "test-agent")

	// The command was queued a long time ago and has only just finished.
	s.mu.Lock()
	s.entries["cmd-slow"].QueuedAt = time.Now().Add(-20 * time.Minute)
	s.mu.Unlock()

	s.Complete("cmd-slow", protocol.CommandResult{ID: "cmd-slow", Type: protocol.CmdUpdateAgent})

	expireNow(s)

	entry, ok := s.Get("cmd-slow")
	if !ok {
		t.Fatal("result was discarded even though it had just arrived")
	}
	if !entry.Done || entry.Result == nil {
		t.Error("entry is present but carries no result")
	}
}

func TestCommandResultStore_ExpiresFinishedEntriesAfterTTL(t *testing.T) {
	s := newCommandResultStore(10 * time.Minute)
	defer s.Stop()

	s.Track("cmd-old", protocol.CmdFetchLogs, "test-agent")
	s.Complete("cmd-old", protocol.CommandResult{ID: "cmd-old", Type: protocol.CmdFetchLogs})

	s.mu.Lock()
	s.entries["cmd-old"].completedAt = time.Now().Add(-11 * time.Minute)
	s.mu.Unlock()

	expireNow(s)

	if _, ok := s.Get("cmd-old"); ok {
		t.Error("a result older than the TTL should have been dropped")
	}
}

// TestCommandResultStore_ExpiresAbandonedEntries covers a command an agent
// never picks up: it must not accumulate forever.
func TestCommandResultStore_ExpiresAbandonedEntries(t *testing.T) {
	s := newCommandResultStore(10 * time.Minute)
	defer s.Stop()

	s.Track("cmd-abandoned", protocol.CmdDiskUsage, "test-agent")

	s.mu.Lock()
	s.entries["cmd-abandoned"].QueuedAt = time.Now().Add(-maxCommandLifetime - time.Minute)
	s.mu.Unlock()

	expireNow(s)

	if _, ok := s.Get("cmd-abandoned"); ok {
		t.Error("an unfinished command past maxCommandLifetime should have been dropped")
	}
}

// TestMaxCommandLifetime_ExceedsAgentSideDeadlines ties the constant to the
// agent timeouts it has to outlast rather than leaving it a bare number.
func TestMaxCommandLifetime_ExceedsAgentSideDeadlines(t *testing.T) {
	// Mirrors internal/agent: updateExecTimeout + commandReportTimeout.
	const agentWorstCase = 10*time.Minute + 30*time.Second

	if maxCommandLifetime <= agentWorstCase {
		t.Errorf("maxCommandLifetime is %v, which does not outlast the agent's worst case of %v",
			maxCommandLifetime, agentWorstCase)
	}
}

func TestCommandResultStore_CompleteOnUnknownIDIsSafe(t *testing.T) {
	s := newCommandResultStore(time.Minute)
	defer s.Stop()

	// Must not panic. It is silently ignored, which is why an entry expiring
	// before its result arrives loses the outcome with no trace.
	s.Complete("cmd-never-tracked", protocol.CommandResult{ID: "cmd-never-tracked"})

	if _, ok := s.Get("cmd-never-tracked"); ok {
		t.Error("Complete should not create an entry for an untracked command")
	}
}
