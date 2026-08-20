package server

import (
	"sync"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

// commandEntry tracks a queued command and its result.
type commandEntry struct {
	ID       string                  `json:"id"`
	Type     protocol.CommandType    `json:"type"`
	AgentID  string                  `json:"agent_id"`
	QueuedAt time.Time               `json:"queued_at"`
	Result   *protocol.CommandResult `json:"result,omitempty"`
	Done     bool                    `json:"done"`

	// completedAt is when the result arrived. Retention is measured from here for
	// finished commands, so a slow command does not have its result discarded
	// moments after reporting it.
	completedAt time.Time
}

// commandResultStore holds in-flight and completed command results with TTL cleanup.
type commandResultStore struct {
	mu      sync.Mutex
	entries map[string]*commandEntry
	ttl     time.Duration
	done    chan struct{}
}

func newCommandResultStore(ttl time.Duration) *commandResultStore {
	s := &commandResultStore{
		entries: make(map[string]*commandEntry),
		ttl:     ttl,
		done:    make(chan struct{}),
	}
	go s.cleanup()
	return s
}

// Track registers a new command that's been queued.
func (s *commandResultStore) Track(id string, cmdType protocol.CommandType, agentID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[id] = &commandEntry{
		ID:       id,
		Type:     cmdType,
		AgentID:  agentID,
		QueuedAt: time.Now(),
	}
}

// Complete stores the result for a tracked command.
func (s *commandResultStore) Complete(id string, result protocol.CommandResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if entry, ok := s.entries[id]; ok {
		entry.Result = &result
		entry.completedAt = time.Now()
		entry.Done = true
	}
}

// Get returns the current state of a command.
func (s *commandResultStore) Get(id string) (*commandEntry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.entries[id]
	if !ok {
		return nil, false
	}
	return entry, true
}

// maxCommandLifetime bounds how long an unfinished command is tracked.
//
// It must exceed the longest a command can legitimately take to report
// (updateExecTimeout plus commandReportTimeout on the agent side), or
// an entry is deleted before its result arrives, Complete finds nothing
// to write to, and the outcome is lost with no trace.
const maxCommandLifetime = 30 * time.Minute

// cleanup removes finished entries ttl after the completed, and unfinished
// entries once they exceed maxCommandLifetime.
func (s *commandResultStore) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.mu.Lock()
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
			s.mu.Unlock()
		case <-s.done:
			return
		}
	}
}

// Stop shuts down the cleanup goroutine.
func (s *commandResultStore) Stop() {
	close(s.done)
}
