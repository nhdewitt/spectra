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
	commandQueues map[string]chan protocol.Command
}

func NewAgentStore() *AgentStore {
	return &AgentStore{
		commandQueues: make(map[string]chan protocol.Command),
	}
}

// Register creates the command queue for an agent.
func (s *AgentStore) Register(hostname string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.commandQueues[hostname]; !exists {
		s.commandQueues[hostname] = make(chan protocol.Command, 10)
	}
}

// Unregister removes an agent's command queue.
func (s *AgentStore) Unregister(hostname string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ch, exists := s.commandQueues[hostname]; exists {
		close(ch)
		delete(s.commandQueues, hostname)
	}
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
		return protocol.Command{}, nil
	case <-ctx.Done():
		return protocol.Command{}, ctx.Err()
	}
}

func (s *AgentStore) Exists(hostname string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.commandQueues[hostname]
	return ok
}
