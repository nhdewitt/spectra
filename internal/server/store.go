package server

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

// AgentStore manages pending commands for agents.
type AgentStore struct {
	mu            sync.Mutex
	agents        map[string]*AgentRecord
	commandQueues map[string]chan protocol.Command
}

type AgentRecord struct {
	ID           string
	Secret       string
	Info         protocol.HostInfo
	RegisteredAt time.Time
	LastSeen     time.Time
}

func NewAgentStore() *AgentStore {
	return &AgentStore{
		agents:        make(map[string]*AgentRecord),
		commandQueues: make(map[string]chan protocol.Command),
	}
}

// Register creates the command queue for an agent.
func (s *AgentStore) Register(agentID, secret string, info protocol.HostInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	if rec, exists := s.agents[agentID]; exists {
		// Re-registration: update info, keep queue
		rec.Info = info
		rec.LastSeen = now
		return
	}

	s.agents[agentID] = &AgentRecord{
		ID:           agentID,
		Secret:       secret,
		Info:         info,
		RegisteredAt: now,
		LastSeen:     now,
	}
	s.commandQueues[agentID] = make(chan protocol.Command, 10)
}

// Unregister removes an agent's command queue.
func (s *AgentStore) Unregister(agentID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ch, exists := s.commandQueues[agentID]; exists {
		close(ch)
		delete(s.commandQueues, agentID)
	}
	delete(s.agents, agentID)
}

// getQueue returns the channel and true if the agent exists.
func (s *AgentStore) getQueue(hostname string) (chan protocol.Command, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch, ok := s.commandQueues[hostname]
	return ch, ok
}

// QueueCommand sends a command to a registered agent.
func (s *AgentStore) QueueCommand(hostname string, cmd protocol.Command) error {
	ch, ok := s.getQueue(hostname)
	if !ok {
		return fmt.Errorf("agent %q not registered", hostname)
	}

	select {
	case ch <- cmd:
		return nil
	default:
		return fmt.Errorf("command queue full for %q", hostname)
	}
}

func (s *AgentStore) WaitForCommand(ctx context.Context, hostname string, timeout time.Duration) (protocol.Command, error) {
	ch, ok := s.getQueue(hostname)
	if !ok {
		return protocol.Command{}, fmt.Errorf("agent %q not registered", hostname)
	}

	select {
	case cmd := <-ch:
		return cmd, nil
	case <-time.After(timeout):
		return protocol.Command{}, fmt.Errorf("timeout waiting for command")
	case <-ctx.Done():
		return protocol.Command{}, ctx.Err()
	}
}

func (s *AgentStore) Exists(agentID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.commandQueues[agentID]
	return ok
}

func (s *AgentStore) Authenticate(agentID, secret string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	rec, ok := s.agents[agentID]
	if !ok {
		return false
	}

	return rec.Secret == secret
}

func (s *AgentStore) TouchLastSeen(agentID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if rec, ok := s.agents[agentID]; ok {
		rec.LastSeen = time.Now()
	}
}

func (s *AgentStore) Remove(agentID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.agents, agentID)
}
