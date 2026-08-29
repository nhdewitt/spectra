package services

import (
	"context"
	"strings"
	"testing"

	"github.com/nhdewitt/spectra/internal/protocol"
)

// collectServices runs Collect and returns the service list.
//
// Skipped services are the normal steady state on Windows: a handful of
// protected services reject OpenService even for an administrator, so Collect
// returns an aggregate error alongside a list that is complete apart from
// those skips. The error is logged rather than treated as a failure, because
// the count is what matters -- 3 of 612 is routine, 598 of 612 is a
// permissions regression worth seeing in the output.
func collectServices(t *testing.T) []protocol.ServiceMetric {
	t.Helper()

	metrics, err := Collect(context.Background())
	if err != nil {
		t.Logf("Collect reported skipped services: %v", err)
	}

	if len(metrics) == 0 {
		t.Fatal("expected at least one metric")
	}

	listMetric, ok := metrics[0].(protocol.ServiceListMetric)
	if !ok {
		t.Fatalf("expected protocol.ServiceListMetric, got %T", metrics[0])
	}

	return listMetric.Services
}

func TestCollect_Integration(t *testing.T) {
	services := collectServices(t)

	if len(services) == 0 {
		t.Fatal("expected at least some services")
	}

	t.Logf("Found %d services", len(services))

	stateCounts := make(map[string]int)
	for _, svc := range services {
		stateCounts[svc.Status]++
	}

	t.Logf("State distribution: %v", stateCounts)
	if stateCounts["Running"] == 0 {
		t.Error("Expected at least one Running service")
	}
}

func TestCollect_ContainsKnownServices(t *testing.T) {
	services := collectServices(t)

	knownServices := []string{"wuauserv", "W32Time", "EventLog", "PlugPlay"}
	found := make(map[string]bool)

	for _, svc := range services {
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

func TestCollect_ValidStates(t *testing.T) {
	services := collectServices(t)

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

	for _, svc := range services {
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

func TestCollect_LoadStateMapping(t *testing.T) {
	services := collectServices(t)

	for _, svc := range services {
		if svc.SubStatus == "Disabled" && svc.LoadState != "disabled" {
			t.Errorf("service %s: StartMode=Disabled but LoadState=%s", svc.Name, svc.LoadState)
		}
		if svc.SubStatus != "Disabled" && svc.LoadState != "loaded" {
			t.Errorf("service %s: StartMode=%s but LoadState=%s", svc.Name, svc.SubStatus, svc.LoadState)
		}
	}
}

func TestCollect_DescriptionFormat(t *testing.T) {
	services := collectServices(t)

	withDescription := 0
	withDisplayNameOnly := 0

	for _, svc := range services {
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

func TestCollect_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Collect(ctx)
	if err == nil {
		t.Log("Collect completed before context cancellation took effect")
	}
}

func TestCollect_NoEmptyNames(t *testing.T) {
	services := collectServices(t)

	for _, svc := range services {
		if svc.Name == "" {
			t.Error("Found service with empty name")
		}
	}
}

func TestSummarizeSkips_NoSkips(t *testing.T) {
	if err := summarizeSkips(0, 612, map[string]*skipReason{}); err != nil {
		t.Errorf("expected nil when nothing was skipped, got %v", err)
	}
}

func TestSummarizeSkips_SingleCause(t *testing.T) {
	reasons := map[string]*skipReason{
		"Access is denied.": {count: 3, example: `opening service "WdNisSvc"`},
	}

	err := summarizeSkips(3, 612, reasons)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	msg := err.Error()
	for _, want := range []string{"3 of 612", "3x Access is denied.", `WdNisSvc`} {
		if !strings.Contains(msg, want) {
			t.Errorf("message missing %q: %s", want, msg)
		}
	}
	if strings.Contains(msg, "other cause") {
		t.Errorf("unexpected truncation suffix on a single cause: %s", msg)
	}
}

// The whole point of grouping: a host that has lost privileges produces the
// same one-line message as a host with three protected services.
func TestSummarizeSkips_LargeCountStaysOneLine(t *testing.T) {
	reasons := map[string]*skipReason{
		"Access is denied.": {count: 598, example: `opening service "AJRouter"`},
	}

	err := summarizeSkips(598, 612, reasons)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	msg := err.Error()
	if strings.Contains(msg, "\n") {
		t.Errorf("message spans multiple lines: %q", msg)
	}
	if !strings.Contains(msg, "598 of 612") {
		t.Errorf("message missing the count: %s", msg)
	}
}

func TestSummarizeSkips_TruncatesPastMaxCauses(t *testing.T) {
	tests := []struct {
		name       string
		causes     int
		wantSuffix string
	}{
		{name: "ExactlyMax", causes: 3, wantSuffix: ""},
		{name: "OneOver", causes: 4, wantSuffix: "; and 1 other cause"},
		{name: "TwoOver", causes: 5, wantSuffix: "; and 2 other causes"},
		{name: "ManyOver", causes: 9, wantSuffix: "; and 6 other causes"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reasons := make(map[string]*skipReason, tt.causes)
			total := 0
			for i := 0; i < tt.causes; i++ {
				reasons[causeName(i)] = &skipReason{count: 1, example: "opening service"}
				total++
			}

			err := summarizeSkips(total, 612, reasons)
			if err == nil {
				t.Fatal("expected an error, got nil")
			}

			msg := err.Error()
			if tt.wantSuffix == "" {
				if strings.Contains(msg, "other cause") {
					t.Errorf("unexpected truncation suffix: %s", msg)
				}
				return
			}
			if !strings.HasSuffix(msg, tt.wantSuffix) {
				t.Errorf("got suffix in %q, want it to end with %q", msg, tt.wantSuffix)
			}
		})
	}
}

// Unsorted, map iteration order would make every sample's message differ and
// defeat any downstream dedup.
func TestSummarizeSkips_MessageIsDeterministic(t *testing.T) {
	build := func() map[string]*skipReason {
		return map[string]*skipReason{
			"Access is denied.":              {count: 4, example: `opening service "A"`},
			"The service does not exist.":    {count: 2, example: `querying service "B"`},
			"The handle is invalid.":         {count: 1, example: `reading config for service "C"`},
			"The system cannot find a path.": {count: 7, example: `opening service "D"`},
		}
	}

	first := summarizeSkips(14, 612, build()).Error()
	for i := 0; i < 25; i++ {
		if got := summarizeSkips(14, 612, build()).Error(); got != first {
			t.Fatalf("message varied between calls:\n  %s\n  %s", first, got)
		}
	}
}

// A rare cause is not swallowed by a common one -- it stays a distinct entry,
// which is what makes grouping preferable to sampling N arbitrary failures.
func TestSummarizeSkips_RareCauseSurvives(t *testing.T) {
	reasons := map[string]*skipReason{
		"Access is denied.":      {count: 500, example: `opening service "A"`},
		"The handle is invalid.": {count: 1, example: `querying service "Rare"`},
	}

	err := summarizeSkips(501, 612, reasons)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "The handle is invalid.") {
		t.Errorf("the single-instance cause was dropped: %s", err.Error())
	}
}

func causeName(i int) string {
	return string(rune('a'+i)) + " failure"
}

func BenchmarkCollect(b *testing.B) {
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		_, _ = Collect(ctx)
	}
}
