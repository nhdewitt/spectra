package collector

import (
	"context"
	"strings"
	"testing"

	"github.com/nhdewitt/spectra/internal/protocol"
)

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
		"Running":         true,
		"Stopped":         true,
		"Paused":          true,
		"StartPending":    true,
		"StopPending":     true,
		"ContinuePending": true,
		"PausePending":    true,
		"Unknown":         true,
	}
	validStartModes := map[string]bool{
		"Auto":     true,
		"Manual":   true,
		"Disabled": true,
		"Boot":     true,
		"System":   true,
		"Unknown":  true,
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

func BenchmarkCollectServices(b *testing.B) {
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		_, _ = CollectServices(ctx)
	}
}
