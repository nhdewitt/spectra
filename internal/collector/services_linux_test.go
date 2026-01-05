//go:build !windows

package collector

import (
	"strings"
	"testing"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func TestParseSystemctlFrom(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedCount int
		validate      func(t *testing.T, metrics []protocol.Metric)
	}{
		{
			name: "Standard Happy Path",
			input: `
ssh.service             loaded active running OpenBSD Secure Shell server
cron.service            loaded active running Regular background program processing daemon
nginx.service           loaded failed failed  A high performance web server
`,
			expectedCount: 3,
			validate: func(t *testing.T, metrics []protocol.Metric) {
				// SSH Check
				ssh := metrics[0].(*protocol.ServiceMetric)
				if ssh.Name != "ssh.service" {
					t.Errorf("Expected ssh.service, got %s", ssh.Name)
				}
				if ssh.Description != "OpenBSD Secure Shell server" {
					t.Errorf("Description mismatch. Got: '%s'", ssh.Description)
				}

				// Nginx Check (Failed State)
				nginx := metrics[2].(*protocol.ServiceMetric)
				if nginx.Status != "failed" || nginx.SubStatus != "failed" {
					t.Errorf("Expected nginx to be failed, got %s/%s", nginx.Status, nginx.SubStatus)
				}
			},
		},
		{
			name: "Filtering Snaps and Loops",
			input: `
ssh.service             loaded active running OpenBSD Secure Shell server
snap-spotify.service    loaded active running Snap Daemon
dev-loop12.device       loaded active plugged Loop Device
snap-core.mount         loaded active mounted Snap Core
docker.service          loaded active running Docker Application Container Engine
`,
			expectedCount: 2, // Should only keep ssh and docker
			validate: func(t *testing.T, metrics []protocol.Metric) {
				if metrics[0].(*protocol.ServiceMetric).Name != "ssh.service" {
					t.Error("First service should be ssh.service")
				}
				if metrics[1].(*protocol.ServiceMetric).Name != "docker.service" {
					t.Error("Second service should be docker.service")
				}
			},
		},
		{
			name: "Missing Description",
			input: `
kmod-static-nodes.service loaded active exited
weird-service.service     loaded active running
`,
			expectedCount: 2,
			validate: func(t *testing.T, metrics []protocol.Metric) {
				s1 := metrics[0].(*protocol.ServiceMetric)
				if s1.Name != "kmod-static-nodes.service" {
					t.Errorf("Expected kmod, got %s", s1.Name)
				}
				if s1.Description != "" {
					t.Errorf("Expected empty description, got '%s'", s1.Description)
				}
			},
		},
		{
			name: "Irregular Whitespace",
			input: `
ssh.service   loaded    active   running   Description   with   extra   spaces
simple.service loaded active running Simple
`,
			expectedCount: 2,
			validate: func(t *testing.T, metrics []protocol.Metric) {
				s1 := metrics[0].(*protocol.ServiceMetric)
				if s1.Description != "Description with extra spaces" {
					t.Errorf("Failed to collapse whitespace: '%s'", s1.Description)
				}
			},
		},
		{
			name: "Empty and Malformed Lines",
			input: `

   
broken-line-with-one-field
broken-line-with two-fields
valid.service loaded active running Valid Service
`,
			expectedCount: 1,
			validate: func(t *testing.T, metrics []protocol.Metric) {
				if metrics[0].(*protocol.ServiceMetric).Name != "valid.service" {
					t.Error("Failed to parse the only valid line")
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

			if len(metrics) != tt.expectedCount {
				t.Errorf("Expected %d metrics, got %d", tt.expectedCount, len(metrics))
			}

			if tt.validate != nil {
				tt.validate(t, metrics)
			}
		})
	}
}
