package server

import (
	"sync"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

// AgentStore manages pending commands for agents
type AgentStore struct {
	mu            sync.Mutex
	commandQueues map[string]chan protocol.Command
}

func NewAgentStore() *AgentStore {
	return &AgentStore{
		commandQueues: make(map[string]chan protocol.Command),
	}
}

func (s *AgentStore) QueueCommand(hostname string, cmd protocol.Command) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	ch, exists := s.commandQueues[hostname]
	if !exists {
		ch = make(chan protocol.Command, 10)
		s.commandQueues[hostname] = ch
	}

	select {
	case ch <- cmd:
		return true
	default:
		return false
	}
}

func (s *AgentStore) WaitForCommand(hostname string, timeout time.Duration) (protocol.Command, bool) {
	s.mu.Lock()
	ch, exists := s.commandQueues[hostname]
	if !exists {
		ch = make(chan protocol.Command, 10)
		s.commandQueues[hostname] = ch
	}
	s.mu.Unlock()

	select {
	case cmd := <-ch:
		return cmd, true
	case <-time.After(timeout):
		return protocol.Command{}, false
	}
}
