package collector

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func TestEncodePowerShell(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"Simple", "Get-Process"},
		{"With Spaces", "Get-Process | Select-Object Name"},
		{"With Newlines", "Get-Process\nGet-Service"},
		{"Unicode", "Write-Host 'Héllo Wörld'"},
		{"Empty", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded := encodePowerShell(tt.input)

			if tt.input != "" && encoded == "" {
				t.Error("expected non-empty encoded output")
			}

			for _, c := range encoded {
				if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
					(c >= '0' && c <= '9') || c == '+' || c == '/' || c == '=') {
					t.Errorf("invalid base64 character: %c", c)
				}
			}
		})
	}
}

func TestEncodePowerShell_Decodable(t *testing.T) {
	input := "Write-Output 'test'"
	encoded := encodePowerShell(input)

	cmd := exec.Command("powershell", "-NoProfile", "-EncodedCommand", encoded)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("PowerShell failed to execute encoded command: %v", err)
	}

	if !strings.Contains(string(out), "test") {
		t.Errorf("expected output to contain 'test', got: %s", string(out))
	}
}

func TestCollectServices_Integration(t *testing.T) {
	ctx := context.Background()
	metrics, err := CollectServices(ctx)
	if err != nil {
		t.Fatalf("CollectServices failed: %v", err)
	}

	if len(metrics) == 0 {
		t.Fatal("expected at least some services")
	}

	listMetric, ok := metrics[0].(*protocol.ServiceListMetric)
	if !ok {
		t.Fatalf("expected *protocol.ServiceListMetric, got %T", metrics[0])
	}

	t.Logf("Found %d services", len(listMetric.Services))

	stateCounts := make(map[string]int)
	for _, svc := range listMetric.Services {
		stateCounts[svc.Status]++
	}

	t.Logf("State distribution: %v", stateCounts)
	if stateCounts["Running"] == 0 {
		t.Error("Expected at least one Running service")
	}
}

func TestCollectServices_ContainsKnownServices(t *testing.T) {
	ctx := context.Background()
	metrics, err := CollectServices(ctx)
	if err != nil {
		t.Fatalf("CollectServices failed: %v", err)
	}

	knownServices := []string{"wuauserv", "W32Time", "EventLog", "PlugPlay"}
	found := make(map[string]bool)

	listMetric, _ := metrics[0].(*protocol.ServiceListMetric)
	for _, svc := range listMetric.Services {
		for _, known := range knownServices {
			if strings.EqualFold(svc.Name, known) {
				found[known] = true
			}
		}
	}

	for _, known := range knownServices {
		if found[known] {
			t.Logf("Found known service: %s", known)
		}
	}

	if len(found) == 0 {
		t.Error("Expected to find at least one known Windows service")
	}
}

func TestCollectServices_ValidStates(t *testing.T) {
	ctx := context.Background()
	metrics, err := CollectServices(ctx)
	if err != nil {
		t.Fatalf("CollectServices failed: %v", err)
	}

	validStates := map[string]bool{
		"Running":          true,
		"Stopped":          true,
		"Paused":           true,
		"Start Pending":    true,
		"Stop Pending":     true,
		"Continue Pending": true,
		"Pause Pending":    true,
		"":                 true,
	}
	validStartModes := map[string]bool{
		"Auto":     true,
		"Manual":   true,
		"Disabled": true,
		"Boot":     true,
		"System":   true,
		"":         true,
	}

	listMetric, _ := metrics[0].(*protocol.ServiceListMetric)
	for _, svc := range listMetric.Services {
		if !validStates[svc.Status] {
			t.Errorf("service %s has unexpected state: %s", svc.Name, svc.Status)
		}
		if !validStartModes[svc.SubStatus] {
			t.Errorf("service %s has unexpected StartMode: %s", svc.Name, svc.SubStatus)
		}
		if svc.LoadState != "loaded" && svc.LoadState != "disabled" {
			t.Errorf("service %s has unexpected LoadState: %s", svc.Name, svc.LoadState)
		}
	}
}

func TestCollectServices_LoadStateMapping(t *testing.T) {
	ctx := context.Background()
	metrics, err := CollectServices(ctx)
	if err != nil {
		t.Fatalf("CollectServices failed: %v", err)
	}

	listMetric, _ := metrics[0].(*protocol.ServiceListMetric)
	for _, svc := range listMetric.Services {
		if svc.SubStatus == "Disabled" && svc.LoadState != "disabled" {
			t.Errorf("service %s: StartMode=Disabled but LoadState=%s", svc.Name, svc.LoadState)
		}
		if svc.SubStatus != "Disabled" && svc.LoadState != "loaded" {
			t.Errorf("service %s: StartMode=%s but LoadState=%s", svc.Name, svc.SubStatus, svc.LoadState)
		}
	}
}

func TestCollectServices_DescriptionFormat(t *testing.T) {
	ctx := context.Background()
	metrics, err := CollectServices(ctx)
	if err != nil {
		t.Fatalf("CollectServices failed: %v", err)
	}

	withDescription := 0
	withDisplayNameOnly := 0

	listMetric, _ := metrics[0].(*protocol.ServiceListMetric)
	for _, svc := range listMetric.Services {
		if svc.Description == "" {
			continue
		}

		if strings.Contains(svc.Description, " - ") {
			withDescription++
		} else {
			withDisplayNameOnly++
		}
	}

	t.Logf("Services with full description: %d", withDescription)
	t.Logf("Services with DisplayName only: %d", withDisplayNameOnly)
}

func TestCollectServices_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := CollectServices(ctx)
	if err == nil {
		t.Log("CollectServices completed before context cancellation took effect")
	}
}

func TestCollectServices_NoEmptyNames(t *testing.T) {
	ctx := context.Background()
	metrics, err := CollectServices(ctx)
	if err != nil {
		t.Fatalf("CollectServices failed: %v", err)
	}

	listMetric := metrics[0].(*protocol.ServiceListMetric)
	for _, svc := range listMetric.Services {
		if svc.Name == "" {
			t.Error("Found service with empty name")
		}
	}
}

func BenchmarkEncodePowerShell_Short(b *testing.B) {
	cmd := "Get-Process"
	b.ReportAllocs()
	for b.Loop() {
		_ = encodePowerShell(cmd)
	}
}

func BenchmarkEncodePowerShell_Long(b *testing.B) {
	cmd := `
		[Console]::OutputEncoding = [System.Text.Encoding]::UTF8;
		Get-CimInstance Win32_Service | 
		Select-Object Name, DisplayName, State, StartMode, Description | 
		ForEach-Object { $_ | ConvertTo-Json -Compress; "" }
	`
	b.ReportAllocs()
	for b.Loop() {
		_ = encodePowerShell(cmd)
	}
}

func BenchmarkCollectServices(b *testing.B) {
	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		_, _ = CollectServices(ctx)
	}
}
