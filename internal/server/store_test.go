package server

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func TestNewAgentStore(t *testing.T) {
	store := NewAgentStore()

	if store == nil {
		t.Fatal("NewAgentStore returned nil")
	}
	if store.commandQueues == nil {
		t.Error("commandQueues map should be initialized")
	}
}

func TestAgentStore_Register(t *testing.T) {
	store := NewAgentStore()

	store.Register("agent-1")

	if !store.Exists("agent-1") {
		t.Error("agent-1 should exist after registration")
	}
}

func TestAgentStore_Register_Idempotent(t *testing.T) {
	store := NewAgentStore()

	store.Register("agent-1")
	store.Register("agent-1")
	store.Register("agent-1")

	if !store.Exists("agent-1") {
		t.Error("agent-1 should exist")
	}

	// Queue a command to verify the channel still works
	err := store.QueueCommand("agent-1", protocol.Command{ID: "cmd-1"})
	if err != nil {
		t.Errorf("QueueCommand failed: %v", err)
	}
}

func TestAgentStore_Register_Multiple(t *testing.T) {
	store := NewAgentStore()

	agents := []string{"agent-1", "agent-2", "agent-3"}
	for _, a := range agents {
		store.Register(a)
	}
	for _, a := range agents {
		if !store.Exists(a) {
			t.Errorf("%s should exist", a)
		}
	}
}

func TestAgentStore_Unregister(t *testing.T) {
	store := NewAgentStore()

	store.Register("agent-1")
	store.Register("agent-2")
	store.Register("agent-3")
	store.Unregister("agent-2")

	if !store.Exists("agent-1") || !store.Exists("agent-3") {
		t.Error("agent-1 and agent-3 should exist after agent-2 is unregistered")
	}
	if store.Exists("agent-2") {
		t.Error("agent-2 should not exist after unregistration")
	}
}

func TestAgentStore_Unregister_NotRegistered(t *testing.T) {
	store := NewAgentStore()

	// Should not panic
	store.Unregister("nonexistent")
}

func TestAgentStore_Exists_NotRegistered(t *testing.T) {
	store := NewAgentStore()

	if store.Exists("nonexistent") {
		t.Error("nonexistent agent should not exist")
	}
}

func TestAgentStore_QueueCommand_Success(t *testing.T) {
	store := NewAgentStore()

	store.Register("agent-1")

	cmd := protocol.Command{ID: "cmd-123", Type: protocol.CmdFetchLogs}
	err := store.QueueCommand("agent-1", cmd)
	if err != nil {
		t.Errorf("QueueCommand failed: %v", err)
	}
}

func TestAgentStore_QueueCommand_NotRegistered(t *testing.T) {
	store := NewAgentStore()

	cmd := protocol.Command{ID: "cmd-123"}
	err := store.QueueCommand("nonexistent", cmd)
	if err == nil {
		t.Error("expected error for unregistered agent")
	}
}

func TestAgentStore_QueueCommand_Full(t *testing.T) {
	store := NewAgentStore()
	store.Register("agent-1")

	for i := range 10 {
		err := store.QueueCommand("agent-1", protocol.Command{ID: "cmd"})
		if err != nil {
			t.Fatalf("QueueCommand %d failed: %v", i, err)
		}
	}

	// Next command should fail
	err := store.QueueCommand("agent-1", protocol.Command{ID: "overflow"})
	if err == nil {
		t.Error("expected error for full queue")
	}
}

func TestAgentStore_WaitForCommand_Success(t *testing.T) {
	store := NewAgentStore()
	store.Register("agent-1")

	go func() {
		time.Sleep(10 * time.Millisecond)
		store.QueueCommand("agent-1", protocol.Command{ID: "cmd-123", Type: protocol.CmdFetchLogs})
	}()

	ctx := context.Background()
	cmd, err := store.WaitForCommand(ctx, "agent-1", 1*time.Second)
	if err != nil {
		t.Fatalf("WaitForCommand failed: %v", err)
	}
	if cmd.ID != "cmd-123" {
		t.Errorf("cmd.ID = %s, want cmd-123", cmd.ID)
	}
}

func TestAgentStore_WaitForCommand_Timeout(t *testing.T) {
	store := NewAgentStore()
	store.Register("agent-1")

	ctx := context.Background()
	cmd, err := store.WaitForCommand(ctx, "agent-1", 10*time.Millisecond)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if cmd.ID != "" {
		t.Errorf("expected empty command on timeout, got %+v", cmd)
	}
}

func TestAgentStore_WaitForCommand_ContextCancel(t *testing.T) {
	store := NewAgentStore()
	store.Register("agent-1")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := store.WaitForCommand(ctx, "agent-1", 1*time.Second)
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestAgentStore_WaitForCommand_ContextTimeout(t *testing.T) {
	store := NewAgentStore()
	store.Register("agent-1")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := store.WaitForCommand(ctx, "agent-1", 1*time.Hour)

	if err != context.DeadlineExceeded {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}
}

func TestAgentStore_WaitForCommand_NotRegistered(t *testing.T) {
	store := NewAgentStore()

	ctx := context.Background()
	_, err := store.WaitForCommand(ctx, "nonexistent", 10*time.Millisecond)

	if err == nil {
		t.Error("expected error for unregistered agent")
	}
}

func TestAgentStore_WaitForCommand_PreQueued(t *testing.T) {
	store := NewAgentStore()
	store.Register("agent-1")

	store.QueueCommand("agent-1", protocol.Command{ID: "cmd-123"})

	ctx := context.Background()
	cmd, err := store.WaitForCommand(ctx, "agent-1", 10*time.Millisecond)
	if err != nil {
		t.Fatalf("WaitForCommand failed: %v", err)
	}
	if cmd.ID != "cmd-123" {
		t.Errorf("cmd.ID = %s, want cmd-123", cmd.ID)
	}
}

func TestAgentStore_QueueAndWait_Order(t *testing.T) {
	store := NewAgentStore()
	store.Register("agent-1")

	for i := range 5 {
		store.QueueCommand("agent-1", protocol.Command{ID: string(rune('A' + i))})
	}

	ctx := context.Background()
	for i := range 5 {
		cmd, err := store.WaitForCommand(ctx, "agent-1", 10*time.Millisecond)
		if err != nil {
			t.Fatalf("WaitForCommand %d failed: %v", i, err)
		}
		expected := string(rune('A' + i))
		if cmd.ID != expected {
			t.Errorf("cmd %d: got %s, want %s", i, cmd.ID, expected)
		}
	}
}

func TestAgentStore_Concurrent_Register(t *testing.T) {
	store := NewAgentStore()

	var wg sync.WaitGroup
	for i := range 100 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			store.Register("agent-1")
		}(i)
	}
	wg.Wait()

	if !store.Exists("agent-1") {
		t.Error("agent-1 should exist")
	}
}

func TestAgentStore_Concurrent_QueueAndWait(t *testing.T) {
	store := NewAgentStore()
	store.Register("agent-1")

	numCommands := 100
	ctx := context.Background()

	var wg sync.WaitGroup

	// Producer
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range numCommands {
			for {
				err := store.QueueCommand("agent-1", protocol.Command{ID: "cmd"})
				if err == nil {
					break
				}
				time.Sleep(1 * time.Millisecond)
			}
		}
	}()

	// Consumer
	received := 0
	wg.Add(1)
	go func() {
		defer wg.Done()
		for received < numCommands {
			cmd, err := store.WaitForCommand(ctx, "agent-1", 100*time.Millisecond)
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

func TestAgentStore_Concurrent_RegisterMultiple(t *testing.T) {
	store := NewAgentStore()

	var wg sync.WaitGroup
	for i := range 100 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			store.Register(fmt.Sprintf("agent-%d", n))
		}(i)
	}
	wg.Wait()

	for i := range 100 {
		if !store.Exists(fmt.Sprintf("agent-%d", i)) {
			t.Errorf("agent-%d should exist", i)
		}
	}
}

func BenchmarkAgentStore_Register(b *testing.B) {
	store := NewAgentStore()

	b.ReportAllocs()
	for b.Loop() {
		store.Register("agent-1")
	}
}

func BenchmarkAgentStore_Exists(b *testing.B) {
	store := NewAgentStore()
	store.Register("agent-1")

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		store.Exists("agent-1")
	}
}

func BenchmarkAgentStore_QueueCommand(b *testing.B) {
	store := NewAgentStore()
	store.Register("agent-1")

	cmd := protocol.Command{ID: "cmd-123", Type: protocol.CmdFetchLogs}
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		store.QueueCommand("agent-1", cmd)
		store.WaitForCommand(ctx, "agent-1", 1*time.Millisecond)
	}
}

func BenchmarkAgentStore_WaitForCommand_Immediate(b *testing.B) {
	store := NewAgentStore()
	store.Register("agent-1")

	cmd := protocol.Command{ID: "cmd-123", Type: protocol.CmdFetchLogs}
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		store.QueueCommand("agent-1", cmd)
		store.WaitForCommand(ctx, "agent-1", 1*time.Second)
	}
}

func BenchmarkAgentStore_WaitForCommand_Timeout(b *testing.B) {
	store := NewAgentStore()
	store.Register("agent-1")

	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		store.WaitForCommand(ctx, "agent-1", 1*time.Nanosecond)
	}
}

func BenchmarkAgentStore_Concurrent_QueueWait(b *testing.B) {
	store := NewAgentStore()
	store.Register("agent-1")

	cmd := protocol.Command{ID: "cmd", Type: protocol.CmdFetchLogs}
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			store.QueueCommand("agent-1", cmd)
			store.WaitForCommand(ctx, "agent-1", 1*time.Millisecond)
		}
	})
}

func BenchmarkAgentStore_Register_ManyAgents(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		store := NewAgentStore()
		for i := range 1000 {
			store.Register(fmt.Sprintf("agent-%d", i))
		}
	}
}
