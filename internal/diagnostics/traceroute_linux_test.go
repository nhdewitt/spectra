//go:build linux

package diagnostics

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRunTraceroute_EmptyTarget(t *testing.T) {
	ctx := context.Background()
	_, err := runTraceroute(ctx, "")
	if err == nil {
		t.Error("expected error for empty target")
	}
	if err.Error() != "target required" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunTraceroute_Localhost(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	if os.Geteuid() != 0 {
		t.Skip("skipping: requires root")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	out, err := runTraceroute(ctx, "127.0.0.1")
	if err != nil {
		t.Fatalf("traceroute failed: %v", err)
	}

	if out == "" {
		t.Error("expected non-empty output")
	}
	if !strings.Contains(out, "127.0.0.1") {
		t.Errorf("output should contain 127.0.0.1:\n%s", out)
	}

	t.Logf("Traceroute output:\n%s", out)
}

func TestRunTraceroute_External(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	if os.Geteuid() != 0 {
		t.Skip("skipping: requires root")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	out, err := runTraceroute(ctx, "8.8.8.8")
	if err != nil {
		t.Fatalf("traceroute failed: %v", err)
	}

	if out == "" {
		t.Error("expected non-empty output")
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		t.Errorf("expected multiple hops, got %d lines", len(lines))
	}

	t.Logf("Traceroute to 8.8.8.8 (%d hops):\n%s", len(lines)-1, out)
}

func TestRunTraceroute_InvalidTarget(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	if os.Geteuid() != 0 {
		t.Skip("skipping: requires root")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	out, err := runTraceroute(ctx, "invalid.host")

	t.Logf("Invalid target - err: %v, output: %s", err, out)
}

func TestRunTraceroute_ContextCancel(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("skipping: requires root")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := runTraceroute(ctx, "8.8.8.8")
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

func TestRunTraceroute_ContextTimeout(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("skipping: requires root")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	time.Sleep(1 * time.Millisecond)

	_, err := runTraceroute(ctx, "8.8.8.8")
	if err == nil {
		t.Error("expected error for timed out context")
	}
}
