package server

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func TestNewCommandQueue(t *testing.T) {
	q := NewCommandQueue()

	if q == nil {
		t.Fatal("NewCommandQueue returned nil")
	}
	if q.queues == nil {
		t.Error("queues map should be initialized")
	}
}

func TestCommandQueue_Send_CreatesChannel(t *testing.T) {
	q := NewCommandQueue()

	cmd := protocol.Command{ID: "cmd-123", Type: protocol.CmdFetchLogs}
	err := q.Send("agent-1", cmd)
	if err != nil {
		t.Errorf("Send failed: %v", err)
	}

	q.mu.Lock()
	_, ok := q.queues["agent-1"]
	q.mu.Unlock()

	if !ok {
		t.Error("channel should be lazily created on Send")
	}
}

func TestCommandQueue_Send_Full(t *testing.T) {
	q := NewCommandQueue()

	for range 10 {
		err := q.Send("agent-1", protocol.Command{ID: "cmd"})
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
	}

	err := q.Send("agent-1", protocol.Command{ID: "overflow"})
	if err == nil {
		t.Error("expected error for full queue")
	}
}

func TestCommandQueue_Wait_CreatesChannel(t *testing.T) {
	q := NewCommandQueue()

	// Wait on a non-existent agent — should create the channel and timeout
	_, err := q.Wait(context.Background(), "agent-1", 10*time.Millisecond)
	if err == nil {
		t.Error("expected timeout error")
	}

	q.mu.Lock()
	_, ok := q.queues["agent-1"]
	q.mu.Unlock()

	if !ok {
		t.Error("channel should be lazily created on Wait")
	}
}

func TestCommandQueue_Wait_Success(t *testing.T) {
	q := NewCommandQueue()

	go func() {
		time.Sleep(10 * time.Millisecond)
		q.Send("agent-1", protocol.Command{ID: "cmd-123", Type: protocol.CmdFetchLogs})
	}()

	cmd, err := q.Wait(context.Background(), "agent-1", 1*time.Second)
	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}
	if cmd.ID != "cmd-123" {
		t.Errorf("cmd.ID = %s, want cmd-123", cmd.ID)
	}
}

func TestCommandQueue_Wait_Timeout(t *testing.T) {
	q := NewCommandQueue()

	_, err := q.Wait(context.Background(), "agent-1", 10*time.Millisecond)
	if err == nil {
		t.Error("expected error on timeout")
	}
}

func TestCommandQueue_Wait_ContextCancel(t *testing.T) {
	q := NewCommandQueue()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := q.Wait(ctx, "agent-1", 1*time.Second)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestCommandQueue_Wait_ContextTimeout(t *testing.T) {
	q := NewCommandQueue()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := q.Wait(ctx, "agent-1", 1*time.Hour)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}
}

func TestCommandQueue_Wait_PreQueued(t *testing.T) {
	q := NewCommandQueue()

	q.Send("agent-1", protocol.Command{ID: "cmd-123"})

	cmd, err := q.Wait(context.Background(), "agent-1", 10*time.Millisecond)
	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}
	if cmd.ID != "cmd-123" {
		t.Errorf("cmd.ID = %s, want cmd-123", cmd.ID)
	}
}

func TestCommandQueue_SendAndWait_Order(t *testing.T) {
	q := NewCommandQueue()

	for i := range 5 {
		q.Send("agent-1", protocol.Command{ID: string(rune('A' + i))})
	}

	for i := range 5 {
		cmd, err := q.Wait(context.Background(), "agent-1", 10*time.Millisecond)
		if err != nil {
			t.Fatalf("Wait %d failed: %v", i, err)
		}
		expected := string(rune('A' + i))
		if cmd.ID != expected {
			t.Errorf("cmd %d: got %s, want %s", i, cmd.ID, expected)
		}
	}
}

func TestCommandQueue_Remove(t *testing.T) {
	q := NewCommandQueue()

	q.Send("agent-1", protocol.Command{ID: "cmd"})
	q.Remove("agent-1")

	q.mu.Lock()
	_, ok := q.queues["agent-1"]
	q.mu.Unlock()

	if ok {
		t.Error("queue should be removed after Remove")
	}
}

func TestCommandQueue_Remove_NotExists(t *testing.T) {
	q := NewCommandQueue()

	// Should not panic
	q.Remove("nonexistent")
}

func TestCommandQueue_Concurrent_SendAndWait(t *testing.T) {
	q := NewCommandQueue()

	numCommands := 100
	ctx := context.Background()

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for range numCommands {
			for {
				err := q.Send("agent-1", protocol.Command{ID: "cmd"})
				if err == nil {
					break
				}
				time.Sleep(1 * time.Millisecond)
			}
		}
	}()

	received := 0
	wg.Add(1)
	go func() {
		defer wg.Done()
		for received < numCommands {
			cmd, err := q.Wait(ctx, "agent-1", 100*time.Millisecond)
			if err == nil && cmd.ID != "" {
				received++
			}
		}
	}()

	wg.Wait()

	if received != numCommands {
		t.Errorf("received %d commands, want %d", received, numCommands)
	}
}

func TestCommandQueue_Concurrent_MultipleAgents(t *testing.T) {
	q := NewCommandQueue()

	var wg sync.WaitGroup
	for i := range 100 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			id := fmt.Sprintf("agent-%d", n)
			q.Send(id, protocol.Command{ID: "cmd"})
		}(i)
	}
	wg.Wait()

	q.mu.Lock()
	count := len(q.queues)
	q.mu.Unlock()

	if count != 100 {
		t.Errorf("expected 100 queues, got %d", count)
	}
}

// --- Benchmarks ---

func BenchmarkCommandQueue_Send(b *testing.B) {
	q := NewCommandQueue()
	cmd := protocol.Command{ID: "cmd-123", Type: protocol.CmdFetchLogs}
	ctx := context.Background()

	b.ReportAllocs()
	for b.Loop() {
		q.Send("agent-1", cmd)
		q.Wait(ctx, "agent-1", 1*time.Millisecond)
	}
}

func BenchmarkCommandQueue_Wait_Immediate(b *testing.B) {
	q := NewCommandQueue()
	cmd := protocol.Command{ID: "cmd-123", Type: protocol.CmdFetchLogs}
	ctx := context.Background()

	b.ReportAllocs()
	for b.Loop() {
		q.Send("agent-1", cmd)
		q.Wait(ctx, "agent-1", 1*time.Second)
	}
}

func BenchmarkCommandQueue_Wait_Timeout(b *testing.B) {
	q := NewCommandQueue()
	ctx := context.Background()

	// Pre-create the channel
	q.Send("agent-1", protocol.Command{ID: "warmup"})
	q.Wait(ctx, "agent-1", 1*time.Millisecond)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		q.Wait(ctx, "agent-1", 1*time.Nanosecond)
	}
}

func BenchmarkCommandQueue_Concurrent(b *testing.B) {
	q := NewCommandQueue()
	cmd := protocol.Command{ID: "cmd", Type: protocol.CmdFetchLogs}
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			q.Send("agent-1", cmd)
			q.Wait(ctx, "agent-1", 1*time.Millisecond)
		}
	})
}
