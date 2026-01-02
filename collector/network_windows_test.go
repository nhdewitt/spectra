//go:build windows

package collector

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/nhdewitt/spectra/metrics"
)

func TestIsIgnoredInterface(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"Ethernet Adapter", false},
		{"Wi-Fi", false},
		{"Intel(R) Gigabit Connection", false},
		{"Microsoft Hyper-V Network Adapter", false},
		{"Teredo Tunneling Pseudo-Interface", true},
		{"WAN Miniport (IP)", true},
		{"Bluetooth Device (Personal Area Network)", false},
		{"Microsoft Kernel Debug Network Adapter", true},
		{"Local Area Connection-0000", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isIgnoredInterface(tt.name)
			if got != tt.want {
				t.Errorf("isIgnoredInterface(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestFormatMAC(t *testing.T) {
	var buf [32]byte
	copy(buf[:], []byte{0x00, 0x11, 0x22, 0xAA, 0xBB, 0xCC})

	tests := []struct {
		name   string
		macArr [32]byte
		len    uint32
		want   string
	}{
		{"Standard MAC", buf, 6, "00:11:22:aa:bb:cc"},
		{"Empty", buf, 0, ""},
		{"Too Long", buf, 33, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatMAC(tt.macArr, tt.len)
			if got != tt.want {
				t.Errorf("formatMAC() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCollectNetwork_Integration(t *testing.T) {
	lastNetStats = nil

	ctx := context.Background()

	data, err := CollectNetwork(ctx)
	if err != nil {
		t.Fatalf("First CollectNetwork call failed: %v", err)
	}
	if len(data) > 0 {
		t.Error("Expected 0 metrics on baseline call, got %d", len(data))
	}

	time.Sleep(1 * time.Second)

	data, err = CollectNetwork(ctx)
	if err != nil {
		t.Fatalf("Second CollectNetwork call failed: %v", err)
	}

	t.Logf("Found %d active network interfaces", len(data))

	for _, m := range data {
		nm, ok := m.(metrics.NetworkMetric)
		if !ok {
			t.Errorf("Expected NetworkMetric, got %T", m)
			continue
		}

		t.Logf("Interface: %s | MAC: %s | Speed: %d", nm.Interface, nm.MAC, nm.Speed)

		if nm.Interface == "" {
			t.Error("Interface name is empty")
		}

		if nm.MAC != "" && !strings.Contains(nm.MAC, ":") {
			t.Errorf("Invalid MAC format: %s", nm.MAC)
		}
	}
}
