package server

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

// CommandQueue manages pending command channels for agents.
// Channels are created lazily on first use (Send or Wait).
type CommandQueue struct {
	mu     sync.Mutex
	queues map[string]chan protocol.Command
}

func NewCommandQueue() *CommandQueue {
	return &CommandQueue{
		queues: make(map[string]chan protocol.Command),
	}
}

// getOrCreate returns the command channel for an agent, creating it if needed.
func (q *CommandQueue) getOrCreate(agentID string) chan protocol.Command {
	q.mu.Lock()
	defer q.mu.Unlock()

	ch, ok := q.queues[agentID]
	if !ok {
		ch = make(chan protocol.Command, 10)
		q.queues[agentID] = ch
	}
	return ch
}

// Send queues a command for an agent.
func (q *CommandQueue) Send(agentID string, cmd protocol.Command) error {
	ch := q.getOrCreate(agentID)

	select {
	case ch <- cmd:
		return nil
	default:
		return fmt.Errorf("command queue full for %q", agentID)
	}
}

// Wait blocks until a command is available or the timeout/context expires.
func (q *CommandQueue) Wait(ctx context.Context, agentID string, timeout time.Duration) (protocol.Command, error) {
	ch := q.getOrCreate(agentID)

	select {
	case cmd := <-ch:
		return cmd, nil
	case <-time.After(timeout):
		return protocol.Command{}, fmt.Errorf("timeout waiting for command")
	case <-ctx.Done():
		return protocol.Command{}, ctx.Err()
	}
}

// Remove cleans up the command channel for a deleted agent.
func (q *CommandQueue) Remove(agentID string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if ch, ok := q.queues[agentID]; ok {
		close(ch)
		delete(q.queues, agentID)
	}
}
