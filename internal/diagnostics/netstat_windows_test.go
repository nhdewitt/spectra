//go:build windows

package diagnostics

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestSplitHostPort(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		expectIP   string
		expectPort uint16
		wantErr    bool
	}{
		{"localhost:80", "127.0.0.1:80", "127.0.0.1", 80, false},
		{"any:22", "0.0.0.0:22", "0.0.0.0", 22, false},
		{"high port", "192.168.1.1:65535", "192.168.1.1", 65535, false},
		{"port 0", "10.0.0.1:0", "10.0.0.1", 0, false},
		{"wildcard", "*:*", "0.0.0.0", 0, false},
		{"wildcard port", "0.0.0.0:*", "0.0.0.0", 0, false},
		{"ipv6 localhost", "[::1]:80", "::1", 80, false},
		{"ipv6 any", "[::]:443", "::", 443, false},
		{"ipv6 full", "[fe80::1]:8080", "fe80::1", 8080, false},
		{"no port", "127.0.0.1", "", 0, true},
		{"empty", "", "", 0, true},
		{"invalid port", "127.0.0.1:abc", "", 0, true},
		{"port too large", "127.0.0.1:99999", "", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip, port, err := splitHostPort(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ip != tt.expectIP {
				t.Errorf("ip: got %s, want %s", ip, tt.expectIP)
			}
			if port != tt.expectPort {
				t.Errorf("port: got %d, want %d", port, tt.expectPort)
			}
		})
	}
}

func TestParseNetstatFrom_TCP(t *testing.T) {
	input := `
Active Connections

  Proto  Local Address          Foreign Address        State           PID
  TCP    0.0.0.0:135            0.0.0.0:0              LISTENING       1234
  TCP    127.0.0.1:8080         192.168.1.100:50000    ESTABLISHED     5678
  TCP    10.0.0.5:443           10.0.0.10:54321        TIME_WAIT       9012
`

	entries, err := parseNetstatFrom(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	// First entry: LISTENING
	e := entries[0]
	if e.Proto != "tcp" {
		t.Errorf("proto: got %s, want tcp", e.Proto)
	}
	if e.LocalAddr != "0.0.0.0" {
		t.Errorf("local addr: got %s, want 0.0.0.0", e.LocalAddr)
	}
	if e.LocalPort != 135 {
		t.Errorf("local port: got %d, want 135", e.LocalPort)
	}
	if e.RemoteAddr != "0.0.0.0" {
		t.Errorf("remote addr: got %s, want 0.0.0.0", e.RemoteAddr)
	}
	if e.RemotePort != 0 {
		t.Errorf("remote port: got %d, want 0", e.RemotePort)
	}
	if e.State != "LISTENING" {
		t.Errorf("state: got %s, want LISTENING", e.State)
	}
	if e.PID != 1234 {
		t.Errorf("pid: got %d, want 1234", e.PID)
	}

	// Second entry: ESTABLISHED
	e = entries[1]
	if e.State != "ESTABLISHED" {
		t.Errorf("state: got %s, want ESTABLISHED", e.State)
	}
	if e.LocalAddr != "127.0.0.1" {
		t.Errorf("local addr: got %s, want 127.0.0.1", e.LocalAddr)
	}
	if e.LocalPort != 8080 {
		t.Errorf("local port: got %d, want 8080", e.LocalPort)
	}
	if e.RemoteAddr != "192.168.1.100" {
		t.Errorf("remote addr: got %s, want 192.168.1.100", e.RemoteAddr)
	}
	if e.RemotePort != 50000 {
		t.Errorf("remote port: got %d, want 50000", e.RemotePort)
	}
	if e.PID != 5678 {
		t.Errorf("pid: got %d, want 5678", e.PID)
	}

	// Third entry: TIME_WAIT
	e = entries[2]
	if e.State != "TIME_WAIT" {
		t.Errorf("state: got %s, want TIME_WAIT", e.State)
	}
}

func TestParseNetstatFrom_UDP(t *testing.T) {
	input := `
Active Connections

  Proto  Local Address          Foreign Address        State           PID
  UDP    0.0.0.0:53             *:*                                    1234
  UDP    127.0.0.1:5353         *:*                                    5678
`

	entries, err := parseNetstatFrom(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	e := entries[0]
	if e.Proto != "udp" {
		t.Errorf("proto: got %s, want udp", e.Proto)
	}
	if e.LocalPort != 53 {
		t.Errorf("local port: got %d, want 53", e.LocalPort)
	}
	if e.State != "" {
		t.Errorf("state: got %q, want empty (UDP has no state)", e.State)
	}
	if e.RemoteAddr != "0.0.0.0" {
		t.Errorf("remote addr: got %s, want 0.0.0.0", e.RemoteAddr)
	}
	if e.PID != 1234 {
		t.Errorf("pid: got %d, want 1234", e.PID)
	}
}

func TestParseNetstatFrom_IPv6(t *testing.T) {
	input := `
Active Connections

  Proto  Local Address          Foreign Address        State           PID
  TCP    [::1]:8080             [::]:0                 LISTENING       1234
  TCP    [::]:443               [::]:0                 LISTENING       5678
  UDP    [::1]:5353             *:*                                    9012
`

	entries, err := parseNetstatFrom(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	// TCP IPv6 localhost
	e := entries[0]
	if e.LocalAddr != "::1" {
		t.Errorf("local addr: got %s, want ::1", e.LocalAddr)
	}
	if e.LocalPort != 8080 {
		t.Errorf("local port: got %d, want 8080", e.LocalPort)
	}

	// TCP IPv6 any
	e = entries[1]
	if e.LocalAddr != "::" {
		t.Errorf("local addr: got %s, want ::", e.LocalAddr)
	}
	if e.LocalPort != 443 {
		t.Errorf("local port: got %d, want 443", e.LocalPort)
	}

	// UDP IPv6
	e = entries[2]
	if e.Proto != "udp" {
		t.Errorf("proto: got %s, want udp", e.Proto)
	}
	if e.LocalAddr != "::1" {
		t.Errorf("local addr: got %s, want ::1", e.LocalAddr)
	}
}

func TestParseNetstatFrom_AllStates(t *testing.T) {
	states := []string{
		"LISTENING", "ESTABLISHED", "TIME_WAIT", "CLOSE_WAIT",
		"FIN_WAIT_1", "FIN_WAIT_2", "SYN_SENT", "SYN_RECEIVED",
		"LAST_ACK", "CLOSING",
	}

	for _, state := range states {
		t.Run(state, func(t *testing.T) {
			input := `
  Proto  Local Address          Foreign Address        State           PID
  TCP    127.0.0.1:8080         192.168.1.1:50000      ` + state + `       1234
`
			entries, err := parseNetstatFrom(strings.NewReader(input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(entries) != 1 {
				t.Fatalf("expected 1 entry, got %d", len(entries))
			}
			if entries[0].State != state {
				t.Errorf("state: got %s, want %s", entries[0].State, state)
			}
		})
	}
}

func TestParseNetstatFrom_Empty(t *testing.T) {
	input := `
Active Connections

  Proto  Local Address          Foreign Address        State           PID
`

	entries, err := parseNetstatFrom(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestParseNetstatFrom_MalformedLines(t *testing.T) {
	input := `
Active Connections

  Proto  Local Address          Foreign Address        State           PID
  TCP    0.0.0.0:80             0.0.0.0:0              LISTENING       1234
  Invalid line here
  TCP    missing:fields
  TCP    0.0.0.0:443            0.0.0.0:0              LISTENING       5678
`

	entries, err := parseNetstatFrom(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should skip malformed lines
	if len(entries) != 2 {
		t.Errorf("expected 2 entries (skipping malformed), got %d", len(entries))
	}
}

func TestParseNetstatFrom_Mixed(t *testing.T) {
	input := `
Active Connections

  Proto  Local Address          Foreign Address        State           PID
  TCP    0.0.0.0:80             0.0.0.0:0              LISTENING       100
  TCP    127.0.0.1:8080         10.0.0.5:12345         ESTABLISHED     200
  UDP    0.0.0.0:53             *:*                                    300
  TCP    [::1]:443              [::]:0                 LISTENING       400
  UDP    [::]:5353              *:*                                    500
`

	entries, err := parseNetstatFrom(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(entries) != 5 {
		t.Fatalf("expected 5 entries, got %d", len(entries))
	}

	// Count by protocol
	tcpCount, udpCount := 0, 0
	for _, e := range entries {
		if e.Proto == "tcp" {
			tcpCount++
		} else if e.Proto == "udp" {
			udpCount++
		}
	}

	if tcpCount != 3 {
		t.Errorf("expected 3 TCP entries, got %d", tcpCount)
	}
	if udpCount != 2 {
		t.Errorf("expected 2 UDP entries, got %d", udpCount)
	}
}

func TestParseNetstatFrom_IgnoresNonTCPUDP(t *testing.T) {
	input := `
  Proto  Local Address          Foreign Address        State           PID
  TCP    0.0.0.0:80             0.0.0.0:0              LISTENING       1234
  ICMP   some line here that should be ignored
  UDP    0.0.0.0:53             *:*                                    5678
`

	entries, err := parseNetstatFrom(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(entries) != 2 {
		t.Errorf("expected 2 entries (TCP and UDP only), got %d", len(entries))
	}
}

func TestGetNetstat_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	entries, err := getNetstat(ctx)
	if err != nil {
		t.Fatalf("getNetstat failed: %v", err)
	}

	t.Logf("Found %d netstat entries", len(entries))

	if len(entries) == 0 {
		t.Error("expected at least some network connections")
	}

	// Count by protocol
	protoCounts := make(map[string]int)
	stateCounts := make(map[string]int)
	for _, e := range entries {
		protoCounts[e.Proto]++
		if e.State != "" {
			stateCounts[e.State]++
		}

		// Validate structure
		if e.Proto == "" {
			t.Error("entry has empty proto")
		}
		if e.Proto != "tcp" && e.Proto != "udp" {
			t.Errorf("unexpected proto: %s", e.Proto)
		}
		if e.LocalAddr == "" {
			t.Error("entry has empty local addr")
		}
	}

	t.Logf("By protocol: %v", protoCounts)
	t.Logf("By state: %v", stateCounts)
}

func TestGetNetstat_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := getNetstat(ctx)
	t.Logf("getNetstat with cancelled context: %v", err)
}

func TestGetNetstat_ContextTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	time.Sleep(1 * time.Millisecond)

	_, err := getNetstat(ctx)
	t.Logf("getNetstat with timeout: %v", err)
}

func BenchmarkSplitHostPort_IPv4(b *testing.B) {
	input := "192.168.1.100:50000"

	b.ReportAllocs()
	for b.Loop() {
		_, _, _ = splitHostPort(input)
	}
}

func BenchmarkSplitHostPort_IPv6(b *testing.B) {
	input := "[fe80::1]:8080"

	b.ReportAllocs()
	for b.Loop() {
		_, _, _ = splitHostPort(input)
	}
}

func BenchmarkSplitHostPort_Wildcard(b *testing.B) {
	input := "*:*"

	b.ReportAllocs()
	for b.Loop() {
		_, _, _ = splitHostPort(input)
	}
}

func BenchmarkParseNetstatFrom_Small(b *testing.B) {
	input := `
  Proto  Local Address          Foreign Address        State           PID
  TCP    0.0.0.0:80             0.0.0.0:0              LISTENING       1234
  TCP    127.0.0.1:8080         192.168.1.1:50000      ESTABLISHED     5678
  UDP    0.0.0.0:53             *:*                                    9012
`

	b.ReportAllocs()
	for b.Loop() {
		_, _ = parseNetstatFrom(strings.NewReader(input))
	}
}

func BenchmarkParseNetstatFrom_Large(b *testing.B) {
	var sb strings.Builder
	sb.WriteString(`
  Proto  Local Address          Foreign Address        State           PID
`)
	for range 500 {
		sb.WriteString(`  TCP    127.0.0.1:8080         192.168.1.1:50000      ESTABLISHED     1234
`)
	}
	input := sb.String()

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = parseNetstatFrom(strings.NewReader(input))
	}
}

func BenchmarkGetNetstat_Integration(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping integration benchmark")
	}

	ctx := context.Background()

	entries, _ := getNetstat(ctx)
	b.Logf("Benchmarking with %d entries", len(entries))

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = getNetstat(ctx)
	}
}
