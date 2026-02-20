package diagnostics

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func TestRunNetworkDiag_UnknownAction(t *testing.T) {
	ctx := context.Background()
	req := protocol.NetworkRequest{
		Action: "invalid_action",
		Target: "localhost",
	}

	_, err := RunNetworkDiag(ctx, req)
	if err == nil {
		t.Error("expected error for unknown action")
	}
	if err.Error() != "unknown network action: invalid_action" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRunNetworkDiag_Ping(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	requireRoot(t)

	ctx := context.Background()
	req := protocol.NetworkRequest{
		Action: "ping",
		Target: "127.0.0.1",
	}

	report, err := RunNetworkDiag(ctx, req)
	if err != nil {
		t.Fatalf("ping failed: %v", err)
	}

	if report.Action != "ping" {
		t.Errorf("action: got %s, want ping", report.Action)
	}
	if report.Target != "127.0.0.1" {
		t.Errorf("target: got %s, want 127.0.0.1", report.Target)
	}
	if len(report.PingResults) == 0 {
		t.Error("expected ping results")
	}

	t.Logf("Ping results: %d responses", len(report.PingResults))
	for i, r := range report.PingResults {
		t.Logf("  [%d] success=%v latency=%v", i, r.Success, r.RTT)
	}
}

func TestRunNetworkDiag_Traceroute(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	requireRoot(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := protocol.NetworkRequest{
		Action: "traceroute",
		Target: "8.8.8.8",
	}

	report, err := RunNetworkDiag(ctx, req)
	if err != nil {
		t.Fatalf("traceroute failed: %v", err)
	}

	if report.Action != "traceroute" {
		t.Errorf("action: got %s, want traceroute", report.Action)
	}
	if report.Target != "8.8.8.8" {
		t.Errorf("target: got %s, want 8.8.8.8", report.Target)
	}
	if len(report.RawOutput) == 0 {
		t.Error("expected non-empty raw output")
	}

	t.Logf("Traceroute output:\n%s", report.RawOutput)
}

func TestRunNetworkDiag_Netstat(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	req := protocol.NetworkRequest{
		Action: "netstat",
	}

	report, err := RunNetworkDiag(ctx, req)
	if err != nil {
		t.Fatalf("netstat failed: %v", err)
	}

	if report.Action != "netstat" {
		t.Errorf("action: got %s, want netstat", report.Action)
	}
	if report.Target != "Local System" {
		t.Errorf("target: got %s, want 'Local System'", report.Target)
	}
	if len(report.Netstat) == 0 {
		t.Error("expected netstat entries")
	}

	t.Logf("Netstat entries: %d", len(report.Netstat))
}

func TestRunNetworkDiag_Connect(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()

	tests := []struct {
		name, target  string
		expectSuccess bool
	}{
		{"google dns", "8.8.8.8:53", true},
		{"cloudflare dns", "1.1.1.1:53", true},
		{"invalid ip", "256.256.256.256:80", false},
		{"unreachable port", "127.0.0.1:59999", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := protocol.NetworkRequest{
				Action: "connect",
				Target: tt.target,
			}

			report, err := RunNetworkDiag(ctx, req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if report.Action != "connect" {
				t.Errorf("action: got %s, want connect", report.Action)
			}
			if len(report.PingResults) != 1 {
				t.Errorf("expected 1 result, got %d", len(report.PingResults))
			}

			result := report.PingResults[0]
			if result.Success != tt.expectSuccess {
				t.Errorf("success: got %v, want %v", result.Success, tt.expectSuccess)
			}

			t.Logf("Connect to %s: success=%v", tt.target, report.PingResults[0].Success)
		})
	}
}

func TestRunNetworkDiag_ContextCancel(t *testing.T) {
	requireRoot(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := protocol.NetworkRequest{
		Action: "ping",
		Target: "127.0.0.1",
	}

	_, err := RunNetworkDiag(ctx, req)

	t.Logf("RunNetworkDiag with cancelled context: %v", err)
}

func TestRunNetworkDiag_ContextTimeout(t *testing.T) {
	requireRoot(t)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	time.Sleep(1 * time.Millisecond)

	req := protocol.NetworkRequest{
		Action: "netstat",
	}

	_, err := RunNetworkDiag(ctx, req)

	t.Logf("RunNetworkDiag with timeout: %v", err)
}

func TestRunNetworkDiag_ReportStructure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	requireRoot(t)

	ctx := context.Background()

	actions := []string{"ping", "traceroute", "netstat", "connect"}
	targets := map[string]string{
		"ping":       "8.8.8.8",
		"traceroute": "8.8.8.8",
		"netstat":    "",
		"connect":    "www.google.com:80",
	}

	for _, action := range actions {
		t.Run(action, func(t *testing.T) {
			req := protocol.NetworkRequest{
				Action: action,
				Target: targets[action],
			}

			report, _ := RunNetworkDiag(ctx, req)

			if report.Action != action {
				t.Errorf("action: got %s, want %s", report.Action, action)
			}

			switch action {
			case "ping":
				for i, p := range report.PingResults {
					t.Logf("  [%d] %s: %v", i, action, p)
				}
			case "traceroute":
				t.Logf("  %s: %v", action, report.RawOutput)
			case "netstat":
				t.Logf("  %s: %v", action, report.Netstat)
			case "connect":
				t.Logf("  %s: %v", action, report.PingResults[0])
			}
		})
	}
}

func TestRunNetworkDiag(t *testing.T) {
	ctx := context.Background()

	req := protocol.NetworkRequest{
		Action: "ping",
		Target: "",
	}

	_, err := RunNetworkDiag(ctx, req)
	lErr := err.Error()
	if strings.Contains(lErr, "permission") || strings.Contains(lErr, "operation not permitted") || strings.Contains(lErr, "exit status 1") {
		t.Skip("skipping RunNetworkDiag test: requires root")
	}
	t.Logf("Ping with empty target: %v", err)
}

func requireRoot(t *testing.T) {
	t.Helper()

	if isPrivileged() {
		return
	}
	t.Skip("requires elevated privileges")
}
