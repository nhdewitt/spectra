//go:build !windows

package collector

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func TestParseSystemctlFrom(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedCount int
		validate      func(t *testing.T, services []protocol.ServiceMetric)
	}{
		{
			name: "Standard Happy Path",
			input: `ssh.service             loaded active running OpenBSD Secure Shell server
cron.service            loaded active running Regular background program processing daemon
nginx.service           loaded failed failed  A high performance web server`,
			expectedCount: 3,
			validate: func(t *testing.T, services []protocol.ServiceMetric) {
				ssh := services[0]
				if ssh.Name != "ssh.service" {
					t.Errorf("Expected ssh.service, got %s", ssh.Name)
				}
				if ssh.LoadState != "loaded" {
					t.Errorf("Expected loaded, got %s", ssh.LoadState)
				}
				if ssh.Status != "active" {
					t.Errorf("Expected active, got %s", ssh.Status)
				}
				if ssh.SubStatus != "running" {
					t.Errorf("Expected running, got %s", ssh.SubStatus)
				}
				if ssh.Description != "OpenBSD Secure Shell server" {
					t.Errorf("Description mismatch. Got: '%s'", ssh.Description)
				}

				nginx := services[2]
				if nginx.Name != "nginx.service" {
					t.Errorf("Expected nginx.service, got %s", nginx.Name)
				}
				if nginx.Status != "failed" || nginx.SubStatus != "failed" {
					t.Errorf("Expected nginx to be failed, got %s/%s", nginx.Status, nginx.SubStatus)
				}
			},
		},
		{
			name: "Filtering Snaps and Loops",
			input: `ssh.service             loaded active running OpenBSD Secure Shell server
snap-spotify.service    loaded active running Snap Daemon
dev-loop12.device       loaded active plugged Loop Device
snap-core.mount         loaded active mounted Snap Core
docker.service          loaded active running Docker Application Container Engine`,
			expectedCount: 2,
			validate: func(t *testing.T, services []protocol.ServiceMetric) {
				if services[0].Name != "ssh.service" {
					t.Errorf("First service should be ssh.service, got %s", services[0].Name)
				}
				if services[1].Name != "docker.service" {
					t.Errorf("Second service should be docker.service, got %s", services[1].Name)
				}
			},
		},
		{
			name: "Missing Description",
			input: `kmod-static-nodes.service loaded active exited
weird-service.service     loaded active running`,
			expectedCount: 2,
			validate: func(t *testing.T, services []protocol.ServiceMetric) {
				if services[0].Name != "kmod-static-nodes.service" {
					t.Errorf("Expected kmod, got %s", services[0].Name)
				}
				if services[0].Description != "" {
					t.Errorf("Expected empty description, got '%s'", services[0].Description)
				}
			},
		},
		{
			name: "Irregular Whitespace Handling",
			input: `ssh.service   loaded    active   running   Description   with   extra   spaces
simple.service loaded active running Simple`,
			expectedCount: 2,
			validate: func(t *testing.T, services []protocol.ServiceMetric) {
				expectedDesc := "Description with extra spaces"
				if services[0].Description != expectedDesc {
					t.Errorf("Expected '%s', got '%s'", expectedDesc, services[0].Description)
				}
			},
		},
		{
			name:          "Empty Input",
			input:         "",
			expectedCount: 0,
			validate:      nil,
		},
		{
			name:          "Only Whitespace",
			input:         "   \n\t\n   ",
			expectedCount: 0,
			validate:      nil,
		},
		{
			name: "Malformed Lines Skipped",
			input: `ssh.service loaded active running SSH
bad
also bad line
docker.service loaded active running Docker`,
			expectedCount: 2,
			validate: func(t *testing.T, services []protocol.ServiceMetric) {
				if services[0].Name != "ssh.service" {
					t.Errorf("Expected ssh.service, got %s", services[0].Name)
				}
				if services[1].Name != "docker.service" {
					t.Errorf("Expected docker.service, got %s", services[1].Name)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(tt.input)

			metrics, err := parseSystemctlFrom(r)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// 2. Validate Container: It returns []protocol.Metric
			// Expecting 1 container metric
			if len(metrics) != 1 {
				t.Fatalf("Expected 1 metric container, got %d", len(metrics))
			}

			// 3. Type Assert to the List Wrapper
			listMetric, ok := metrics[0].(protocol.ServiceListMetric)
			if !ok {
				t.Fatalf("Expected *protocol.ServiceListMetric, got %T", metrics[0])
			}

			// 4. Validate the Inner List
			if len(listMetric.Services) != tt.expectedCount {
				t.Errorf("Expected count %d, got %d", tt.expectedCount, len(listMetric.Services))
			}

			if tt.validate != nil {
				tt.validate(t, listMetric.Services)
			}
		})
	}
}

func TestParseSystemctlFrom_AllStates(t *testing.T) {
	input := `active.service loaded active running Active Service
inactive.service loaded inactive dead Inactive Service
failed.service loaded failed failed Failed Service
notfound.service not-found inactive dead Not Found Service`

	r := strings.NewReader(input)
	metrics, err := parseSystemctlFrom(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	listMetric := metrics[0].(protocol.ServiceListMetric)

	expected := []struct {
		name      string
		loadState string
		status    string
		subStatus string
	}{
		{"active.service", "loaded", "active", "running"},
		{"inactive.service", "loaded", "inactive", "dead"},
		{"failed.service", "loaded", "failed", "failed"},
		{"notfound.service", "not-found", "inactive", "dead"},
	}

	for i, exp := range expected {
		svc := listMetric.Services[i]
		if svc.Name != exp.name {
			t.Errorf("[%d] Name: expected %s, got %s", i, exp.name, svc.Name)
		}
		if svc.LoadState != exp.loadState {
			t.Errorf("[%d] LoadState: expected %s, got %s", i, exp.loadState, svc.LoadState)
		}
		if svc.Status != exp.status {
			t.Errorf("[%d] Status: expected %s, got %s", i, exp.status, svc.Status)
		}
		if svc.SubStatus != exp.subStatus {
			t.Errorf("[%d] SubStatus: expected %s, got %s", i, exp.subStatus, svc.SubStatus)
		}
	}
}

func TestIntern(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"loaded", "loaded"},
		{"not-found", "not-found"},
		{"active", "active"},
		{"inactive", "inactive"},
		{"running", "running"},
		{"dead", "dead"},
		{"failed", "failed"},
		{"exited", "exited"},
		{"unknown", "unknown"},
		{"custom-state", "custom-state"},
	}

	for _, tt := range tests {
		result := intern([]byte(tt.input))
		if result != tt.expected {
			t.Errorf("intern(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestIntern_ReturnsSamePointer(t *testing.T) {
	// Interned strings should return the same pointer
	s1 := intern([]byte("loaded"))
	s2 := intern([]byte("loaded"))

	if s1 != s2 {
		t.Error("interned strings should be equal")
	}
}

func TestParseSystemctlFrom_LongDescription(t *testing.T) {
	input := `long.service loaded active running This is a very long description that spans many words and should be preserved exactly as provided by systemctl`

	r := strings.NewReader(input)
	metrics, err := parseSystemctlFrom(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	listMetric := metrics[0].(protocol.ServiceListMetric)
	svc := listMetric.Services[0]

	expected := "This is a very long description that spans many words and should be preserved exactly as provided by systemctl"
	if svc.Description != expected {
		t.Errorf("Description mismatch:\nExpected: %s\nGot: %s", expected, svc.Description)
	}
}

func TestParseSystemctlFrom_SpecialCharactersInDescription(t *testing.T) {
	input := `special.service loaded active running D-Bus (Desktop Bus) message broker`

	r := strings.NewReader(input)
	metrics, err := parseSystemctlFrom(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	listMetric := metrics[0].(protocol.ServiceListMetric)
	svc := listMetric.Services[0]

	expected := "D-Bus (Desktop Bus) message broker"
	if svc.Description != expected {
		t.Errorf("Description mismatch:\nExpected: %s\nGot: %s", expected, svc.Description)
	}
}

func TestCollectServices_Integration(t *testing.T) {
	if _, err := exec.LookPath("systemctl"); err != nil {
		t.Skip("systemctl not available")
	}

	ctx := context.Background()
	metrics, err := CollectServices(ctx)
	if err != nil {
		t.Fatalf("CollectServices failed: %v", err)
	}

	if len(metrics) != 1 {
		t.Fatalf("Expected 1 metric, got %d", len(metrics))
	}

	listMetric, ok := metrics[0].(protocol.ServiceListMetric)
	if !ok {
		t.Fatalf("Expected *protocol.ServiceListMetric, got %T", metrics[0])
	}

	t.Logf("Found %d services", len(listMetric.Services))

	if len(listMetric.Services) == 0 {
		t.Error("expected at least some services")
	}

	statusCounts := make(map[string]int)
	loadStateCounts := make(map[string]int)
	for _, svc := range listMetric.Services {
		statusCounts[svc.Status]++
		loadStateCounts[svc.LoadState]++
	}

	t.Logf("Status distribution: %v", statusCounts)
	t.Logf("LoadState distribution: %v", loadStateCounts)

	if statusCounts["active"] == 0 {
		t.Error("expected at least one active service")
	}
}

func TestCollectServices_ContextCancel(t *testing.T) {
	if _, err := exec.LookPath("systemctl"); err != nil {
		t.Skip("systemctl not available")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := CollectServices(ctx)
	if err == nil {
		t.Log("CollectServices completed before context cancellation took effect")
	}
}

func TestMakeServiceCollector_EmptyPath(t *testing.T) {
	col := MakeServiceCollector("")
	metrics, err := col(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if metrics != nil {
		t.Errorf("expected nil metrics for empty path, got %v", metrics)
	}
}

func BenchmarkParseSystemctlFrom_Small(b *testing.B) {
	input := `ssh.service loaded active running OpenBSD Secure Shell server
cron.service loaded active running Regular background program processing daemon
docker.service loaded active running Docker Application Container Engine`

	b.ReportAllocs()
	for b.Loop() {
		r := strings.NewReader(input)
		_, _ = parseSystemctlFrom(r)
	}
}

func BenchmarkParseSystemctlFrom_Medium(b *testing.B) {
	var sb strings.Builder
	for i := 0; i < 50; i++ {
		sb.WriteString("service")
		sb.WriteString(string(rune('0' + i%10)))
		sb.WriteString(".service loaded active running Service description number ")
		sb.WriteString(string(rune('0' + i%10)))
		sb.WriteByte('\n')
	}
	input := sb.String()

	b.ReportAllocs()
	for b.Loop() {
		r := strings.NewReader(input)
		_, _ = parseSystemctlFrom(r)
	}
}

func BenchmarkParseSystemctlFrom_Large(b *testing.B) {
	var sb strings.Builder
	for i := range 200 {
		sb.WriteString("service-with-longer-name-")
		sb.WriteString(string(rune('0' + i/100)))
		sb.WriteString(string(rune('0' + (i/10)%10)))
		sb.WriteString(string(rune('0' + i%10)))
		sb.WriteString(".service loaded active running A longer description for this particular service\n")
	}
	input := sb.String()

	b.ReportAllocs()
	for b.Loop() {
		r := strings.NewReader(input)
		_, _ = parseSystemctlFrom(r)
	}
}

func BenchmarkParseSystemctlFrom_WithFiltering(b *testing.B) {
	var sb strings.Builder
	for i := range 100 {
		if i%3 == 0 {
			sb.WriteString("snap-package")
			sb.WriteString(string(rune('0' + i%10)))
			sb.WriteString(".service loaded active running Snap\n")
		} else if i%5 == 0 {
			sb.WriteString("dev-loop")
			sb.WriteString(string(rune('0' + i%10)))
			sb.WriteString(".device loaded active plugged Loop\n")
		} else {
			sb.WriteString("real-service-")
			sb.WriteString(string(rune('0' + i%10)))
			sb.WriteString(".service loaded active running Real Service\n")
		}
	}
	input := sb.String()

	b.ReportAllocs()
	for b.Loop() {
		r := strings.NewReader(input)
		_, _ = parseSystemctlFrom(r)
	}
}

func BenchmarkIntern_Hit(b *testing.B) {
	input := []byte("loaded")
	b.ReportAllocs()
	for b.Loop() {
		_ = intern(input)
	}
}

func BenchmarkIntern_Miss(b *testing.B) {
	input := []byte("unknown-state")
	b.ReportAllocs()
	for b.Loop() {
		_ = intern(input)
	}
}

func BenchmarkCollectServices(b *testing.B) {
	if _, err := exec.LookPath("systemctl"); err != nil {
		b.Skip("systemctl not available")
	}

	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		_, _ = CollectServices(ctx)
	}
}
