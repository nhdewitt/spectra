//go:build !windows

package diagnostics

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestParseIPv4Hex(t *testing.T) {
	tests := []struct {
		name, input, expected string
		wantErr               bool
	}{
		{"localhost", "0100007F", "127.0.0.1", false},
		{"any", "00000000", "0.0.0.0", false},
		{"192.168.1.1", "0101A8C0", "192.168.1.1", false},
		{"10.0.0.1", "0100000A", "10.0.0.1", false},
		{"255.255.255.255", "FFFFFFFF", "255.255.255.255", false},
		{"invalid hex", "ZZZZZZZZ", "", true},
		{"too short", "0100007", "", true},
		{"too long", "0100007F00", "", true},
		{"empty", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip, err := parseIPv4Hex(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ip.String() != tt.expected {
				t.Errorf("got %s, want %s", ip.String(), tt.expected)
			}
		})
	}
}

func TestParseIPv6Hex(t *testing.T) {
	tests := []struct {
		name, input, expected string
		wantErr               bool
	}{
		{
			name:     "localhost",
			input:    "00000000000000000000000001000000",
			expected: "::1",
			wantErr:  false,
		},
		{
			name:     "any",
			input:    "00000000000000000000000000000000",
			expected: "::",
			wantErr:  false,
		},
		{
			name:     "invalid hex",
			input:    "ZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZ",
			expected: "",
			wantErr:  true,
		},
		{
			name:     "too short",
			input:    "0000000000000000000000000100000",
			expected: "",
			wantErr:  true,
		},
		{
			name:     "empty",
			input:    "",
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip, err := parseIPv6Hex(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ip.String() != tt.expected {
				t.Errorf("got %s, want %s", ip.String(), tt.expected)
			}
		})
	}
}

func TestParseAddr(t *testing.T) {
	tests := []struct {
		name, input, expectIP string
		expectPort            uint16
		wantErr               bool
	}{
		{"localhost:8080", "0100007F:1F90", "127.0.0.1", 8080, false},
		{"any:22", "00000000:0016", "0.0.0.0", 22, false},
		{"any:80", "00000000:0050", "0.0.0.0", 80, false},
		{"any:443", "00000000:01BB", "0.0.0.0", 443, false},
		{"high port", "0100007F:FFFF", "127.0.0.1", 65535, false},
		{"port 0", "0100007F:0000", "127.0.0.1", 0, false},
		{"ipv6 localhost", "00000000000000000000000001000000:0050", "::1", 80, false},
		{"no colon", "0100007F1F90", "", 0, true},
		{"empty", "", "", 0, true},
		{"invalid ip hex", "ZZZZZZZZ:0050", "", 0, true},
		{"invalid port hex", "0100007F:ZZZZ", "", 0, true},
		{"bad ip length", "01007F:0050", "", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip, port, err := parseAddr(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
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

func TestParseProcNetFrom_TCP(t *testing.T) {
	input := `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 0100007F:1F90 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 12345 1 0000000000000000 100 0 0 10 0
   1: 0100007F:0050 0101A8C0:C350 01 00000000:00000000 00:00000000 00000000     0        0 12346 1 0000000000000000 100 0 0 10 0`

	entries, err := parseProcNetFrom(strings.NewReader(input), "tcp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	e := entries[0] // LISTEN localhost:8080
	if e.Proto != "tcp" {
		t.Errorf("proto: got %s, want tcp", e.Proto)
	}
	if e.LocalAddr != "127.0.0.1" {
		t.Errorf("local addr: got %s, want 127.0.0.1", e.LocalAddr)
	}
	if e.LocalPort != 8080 {
		t.Errorf("local port: got %d, want 8080", e.LocalPort)
	}
	if e.RemoteAddr != "0.0.0.0" {
		t.Errorf("remote addr: got %s, want 0.0.0.0", e.RemoteAddr)
	}
	if e.RemotePort != 0 {
		t.Errorf("remote port: got %d, want 0", e.RemotePort)
	}
	if e.State != "LISTEN" {
		t.Errorf("state: got %s, want LISTEN", e.State)
	}
	if e.User != "1000" {
		t.Errorf("user: got %s, want 1000", e.User)
	}

	e = entries[1] // ESTABLISHED
	if e.State != "ESTABLISHED" {
		t.Errorf("state: got %s, want ESTABLISHED", e.State)
	}
	if e.RemoteAddr != "192.168.1.1" {
		t.Errorf("remote addr: got %s, want 192.168.1.1", e.RemoteAddr)
	}
	if e.RemotePort != 50000 {
		t.Errorf("remote port: got %d, want 50000", e.RemotePort)
	}
}

func TestParseProcNetFrom_UDP(t *testing.T) {
	input := `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 00000000:0035 00000000:0000 07 00000000:00000000 00:00000000 00000000     0        0 12345 1 0000000000000000 100 0 0 10 0`

	entries, err := parseProcNetFrom(strings.NewReader(input), "udp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
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
}

func TestParseProcNetFrom_TCP6(t *testing.T) {
	input := `  sl  local_address                         remote_address                        st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 00000000000000000000000001000000:0050 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 12345 1 0000000000000000 100 0 0 10 0`

	entries, err := parseProcNetFrom(strings.NewReader(input), "tcp6")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	e := entries[0]
	if e.Proto != "tcp6" {
		t.Errorf("proto: got %s, want tcp6", e.Proto)
	}
	if e.LocalAddr != "::1" {
		t.Errorf("local addr: got %s, want ::1", e.LocalAddr)
	}
	if e.LocalPort != 80 {
		t.Errorf("local port: got %d, want 80", e.LocalPort)
	}
	if e.State != "LISTEN" {
		t.Errorf("state: got %s, want LISTEN", e.State)
	}
}

func TestParseProcNetFrom_AllStates(t *testing.T) {
	// Test all TCP states
	states := map[string]string{
		"01": "ESTABLISHED",
		"02": "SYN_SENT",
		"03": "SYN_RECV",
		"04": "FIN_WAIT1",
		"05": "FIN_WAIT2",
		"06": "TIME_WAIT",
		"07": "CLOSE",
		"08": "CLOSE_WAIT",
		"09": "LAST_ACK",
		"0A": "LISTEN",
		"0B": "CLOSING",
	}

	for hex, expected := range states {
		t.Run(expected, func(t *testing.T) {
			input := `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 0100007F:1F90 00000000:0000 ` + hex + ` 00000000:00000000 00:00000000 00000000  1000        0 12345 1 0000000000000000 100 0 0 10 0`

			entries, err := parseProcNetFrom(strings.NewReader(input), "tcp")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(entries) != 1 {
				t.Fatalf("expected 1 entry, got %d", len(entries))
			}
			if entries[0].State != expected {
				t.Errorf("state: got %s, want %s", entries[0].State, expected)
			}
		})
	}
}

func TestParseProcNetFrom_UnknownState(t *testing.T) {
	input := `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 0100007F:1F90 00000000:0000 FF 00000000:00000000 00:00000000 00000000  1000        0 12345 1 0000000000000000 100 0 0 10 0`

	entries, err := parseProcNetFrom(strings.NewReader(input), "tcp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].State != "UNKNOWN" {
		t.Errorf("state: got %s, want UNKNOWN", entries[0].State)
	}
}

func TestParseProcNetFrom_Empty(t *testing.T) {
	input := `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode`

	entries, err := parseProcNetFrom(strings.NewReader(input), "tcp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestParseProcNetFrom_MalformedLines(t *testing.T) {
	input := `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 0100007F:1F90 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 12345 1 0000000000000000 100 0 0 10 0
   1: invalid line
   2: too:few:fields
   3: ZZZZZZZZ:1F90 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 12345 1 0000000000000000 100 0 0 10 0
   4: 0100007F:1F90 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 12346 1 0000000000000000 100 0 0 10 0`

	entries, err := parseProcNetFrom(strings.NewReader(input), "tcp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should skip malformed lines and parse valid ones
	if len(entries) != 2 {
		t.Errorf("expected 2 entries (skipping malformed), got %d", len(entries))
	}
}

func TestParseProcNetFrom_HeaderOnly(t *testing.T) {
	input := `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
`

	entries, err := parseProcNetFrom(strings.NewReader(input), "tcp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
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

	// Should have at least some connections on any system
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
	if err != context.Canceled {
		t.Logf("getNetstat with cancelled context: %v", err)
	}
}

func TestGetNetstat_ContextTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	time.Sleep(1 * time.Millisecond)

	_, err := getNetstat(ctx)
	t.Logf("getNetstat with timeout: %v", err)
}

func TestTcpStatesMap(t *testing.T) {
	// Verify all expected states are present
	expectedStates := []string{
		"ESTABLISHED", "SYN_SENT", "SYN_RECV", "FIN_WAIT1", "FIN_WAIT2",
		"TIME_WAIT", "CLOSE", "CLOSE_WAIT", "LAST_ACK", "LISTEN", "CLOSING",
	}

	for _, state := range expectedStates {
		found := false
		for _, v := range tcpStates {
			if v == state {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing state in tcpStates map: %s", state)
		}
	}

	// Verify hex keys are valid
	for k := range tcpStates {
		if len(k) != 2 {
			t.Errorf("invalid hex key length: %s", k)
		}
	}
}

func BenchmarkParseIPv4Hex(b *testing.B) {
	input := "0100007F"

	b.ReportAllocs()
	for b.Loop() {
		_, _ = parseIPv4Hex(input)
	}
}

func BenchmarkParseIPv6Hex(b *testing.B) {
	input := "00000000000000000000000001000000"

	b.ReportAllocs()
	for b.Loop() {
		_, _ = parseIPv6Hex(input)
	}
}

func BenchmarkParseAddr_IPv4(b *testing.B) {
	input := "0100007F:1F90"

	b.ReportAllocs()
	for b.Loop() {
		_, _, _ = parseAddr(input)
	}
}

func BenchmarkParseAddr_IPv6(b *testing.B) {
	input := "00000000000000000000000001000000:0050"

	b.ReportAllocs()
	for b.Loop() {
		_, _, _ = parseAddr(input)
	}
}

func BenchmarkParseProcNetFrom_Small(b *testing.B) {
	input := `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 0100007F:1F90 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 12345 1 0000000000000000 100 0 0 10 0
   1: 0100007F:0050 0101A8C0:C350 01 00000000:00000000 00:00000000 00000000     0        0 12346 1 0000000000000000 100 0 0 10 0
   2: 00000000:0016 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 12347 1 0000000000000000 100 0 0 10 0`

	b.ReportAllocs()
	for b.Loop() {
		_, _ = parseProcNetFrom(strings.NewReader(input), "tcp")
	}
}

func BenchmarkParseProcNetFrom_Large(b *testing.B) {
	var sb strings.Builder
	sb.WriteString(`  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
`)
	for i := 0; i < 500; i++ {
		sb.WriteString(`   0: 0100007F:1F90 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 12345 1 0000000000000000 100 0 0 10 0
`)
	}
	input := sb.String()

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = parseProcNetFrom(strings.NewReader(input), "tcp")
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
