//go:build freebsd

package diagnostics

import (
	"strings"
	"testing"
)

const sockstatOutput = `USER     COMMAND     PID FD PROTO LOCAL ADDRESS         FOREIGN ADDRESS       STATE
nhdewitt ThreadPool 7173 35 tcp4  172.22.41.228:39650   20.42.73.30:443       ESTABLISHED
nhdewitt firefox    7118 37 tcp4  172.22.41.228:35158   34.107.243.93:443     ESTABLISHED
root     sshd       4297  6 tcp6  *:22                  *:*                   LISTEN
root     sshd       4297  7 tcp4  *:22                  *:*                   LISTEN
ntpd     ntpd       4179 20 udp6  *:123                 *:*                   ??
ntpd     ntpd       4179 23 udp4  172.22.41.228:123     *:*                   ??
ntpd     ntpd       4179 22 udp6  fe80::3213:8bff:fe85:1234 *:*              ??
??       ??           ?? ?? tcp4  172.22.41.228:37820   160.79.104.10:443     ESTABLISHED
`

func TestGetNetstatFrom(t *testing.T) {
	entries, err := getNetstatFrom(strings.NewReader(sockstatOutput))
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) == 0 {
		t.Fatal("expected entries")
	}

	t.Logf("parsed %d entries", len(entries))
}

func TestGetNetstatFromTCP(t *testing.T) {
	entries, err := getNetstatFrom(strings.NewReader(sockstatOutput))
	if err != nil {
		t.Fatal(err)
	}

	// First entry: established TCP
	e := entries[0]
	if e.Proto != "tcp4" {
		t.Errorf("proto = %q, want tcp4", e.Proto)
	}
	if e.LocalAddr != "172.22.41.228" {
		t.Errorf("local addr = %q, want 172.22.41.228", e.LocalAddr)
	}
	if e.LocalPort != 39650 {
		t.Errorf("local port = %d, want 39650", e.LocalPort)
	}
	if e.RemoteAddr != "20.42.73.30" {
		t.Errorf("remote addr = %q, want 20.42.73.30", e.RemoteAddr)
	}
	if e.RemotePort != 443 {
		t.Errorf("remote port = %d, want 443", e.RemotePort)
	}
	if e.State != "ESTABLISHED" {
		t.Errorf("state = %q, want ESTABLISHED", e.State)
	}
	if e.User != "nhdewitt" {
		t.Errorf("user = %q, want nhdewitt", e.User)
	}
}

func TestGetNetstatFromListener(t *testing.T) {
	entries, err := getNetstatFrom(strings.NewReader(sockstatOutput))
	if err != nil {
		t.Fatal(err)
	}

	// Find sshd listener on tcp6
	var sshd *struct{ found bool }
	for _, e := range entries {
		if e.Proto == "tcp6" && e.LocalPort == 22 && e.State == "LISTEN" {
			sshd = &struct{ found bool }{true}
			if e.LocalAddr != "*" {
				t.Errorf("listener local addr = %q, want *", e.LocalAddr)
			}
			if e.RemoteAddr != "*" {
				t.Errorf("listener remote addr = %q, want *", e.RemoteAddr)
			}
			if e.RemotePort != 0 {
				t.Errorf("listener remote port = %d, want 0", e.RemotePort)
			}
			break
		}
	}
	if sshd == nil {
		t.Error("sshd tcp6 LISTEN entry not found")
	}
}

func TestGetNetstatFromUDPState(t *testing.T) {
	entries, err := getNetstatFrom(strings.NewReader(sockstatOutput))
	if err != nil {
		t.Fatal(err)
	}

	for _, e := range entries {
		if strings.HasPrefix(e.Proto, "udp") && e.State != "" {
			t.Errorf("UDP entry has state %q, want empty", e.State)
			break
		}
	}
}

func TestGetNetstatFromUnknownUser(t *testing.T) {
	entries, err := getNetstatFrom(strings.NewReader(sockstatOutput))
	if err != nil {
		t.Fatal(err)
	}

	// The ?? user entry should still parse
	var found bool
	for _, e := range entries {
		if e.User == "??" {
			found = true
			if e.State != "ESTABLISHED" {
				t.Errorf("?? user state = %q, want ESTABLISHED", e.State)
			}
			break
		}
	}
	if !found {
		t.Error("entry with ?? user not found")
	}
}

func TestGetNetstatFromEmpty(t *testing.T) {
	header := "USER     COMMAND     PID FD PROTO LOCAL ADDRESS         FOREIGN ADDRESS       STATE\n"
	entries, err := getNetstatFrom(strings.NewReader(header))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("got %d entries for header-only input, want 0", len(entries))
	}
}

// --- parseAddr tests ---

func TestParseAddr(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantAddr string
		wantPort uint16
		wantErr  bool
	}{
		{"ipv4", "172.22.41.228:39650", "172.22.41.228", 39650, false},
		{"wildcard both", "*:*", "*", 0, false},
		{"wildcard port", "*:22", "*", 22, false},
		{"wildcard addr", "*:*", "*", 0, false},
		{"ipv6 simple", "::1:123", "::1", 123, false},
		{"ipv6 link local", "fe80::3213:8bff:fe85:1234", "fe80::3213:8bff:fe85", 1234, false},
		{"ipv6 scope", "fe80::1%lo0:123", "fe80::1%lo0", 123, false},
		{"no colon", "garbage", "", 0, true},
		{"bad port", "127.0.0.1:notaport", "", 0, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			addr, port, err := parseAddr(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error for %q", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if addr != tc.wantAddr {
				t.Errorf("addr = %q, want %q", addr, tc.wantAddr)
			}
			if port != tc.wantPort {
				t.Errorf("port = %d, want %d", port, tc.wantPort)
			}
		})
	}
}
