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
		validate      func(t *testing.T, services []protocol.ServiceMetric)
	}{
		{
			name: "Standard Happy Path",
			input: `
ssh.service             loaded active running OpenBSD Secure Shell server
cron.service            loaded active running Regular background program processing daemon
nginx.service           loaded failed failed  A high performance web server
`,
			expectedCount: 3,
			validate: func(t *testing.T, services []protocol.ServiceMetric) {
				// 1. SSH Check
				ssh := services[0]
				if ssh.Name != "ssh.service" {
					t.Errorf("Expected ssh.service, got %s", ssh.Name)
				}
				if ssh.LoadState != "loaded" {
					t.Errorf("Expected loaded, got %s", ssh.LoadState)
				}
				if ssh.Description != "OpenBSD Secure Shell server" {
					t.Errorf("Description mismatch. Got: '%s'", ssh.Description)
				}

				// 2. Nginx Check (Failed State)
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
			input: `
ssh.service             loaded active running OpenBSD Secure Shell server
snap-spotify.service    loaded active running Snap Daemon
dev-loop12.device       loaded active plugged Loop Device
snap-core.mount         loaded active mounted Snap Core
docker.service          loaded active running Docker Application Container Engine
`,
			expectedCount: 2,
			validate: func(t *testing.T, services []protocol.ServiceMetric) {
				if len(services) != 2 {
					t.Fatalf("Expected 2 services (ssh, docker), got %d", len(services))
				}
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
			input: `
kmod-static-nodes.service loaded active exited
weird-service.service     loaded active running
`,
			expectedCount: 2,
			validate: func(t *testing.T, services []protocol.ServiceMetric) {
				s1 := services[0]
				if s1.Name != "kmod-static-nodes.service" {
					t.Errorf("Expected kmod, got %s", s1.Name)
				}
				// Your code joins fields[4:] with spaces. If fields has length 4, the loop over [4:] is empty.
				// Description should be empty string.
				if s1.Description != "" {
					t.Errorf("Expected empty description, got '%s'", s1.Description)
				}
			},
		},
		{
			name: "Irregular Whitespace Handling",
			// bytes.Fields handles multiple spaces automatically
			input: `
ssh.service   loaded    active   running   Description   with   extra   spaces
simple.service loaded active running Simple
`,
			expectedCount: 2,
			validate: func(t *testing.T, services []protocol.ServiceMetric) {
				s1 := services[0]
				expectedDesc := "Description with extra spaces"
				if s1.Description != expectedDesc {
					t.Errorf("Expected '%s', got '%s'", expectedDesc, s1.Description)
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
			listMetric, ok := metrics[0].(*protocol.ServiceListMetric)
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
