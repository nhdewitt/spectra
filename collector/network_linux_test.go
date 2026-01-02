//go:build !windows

package collector

import (
	"fmt"
	"strings"
	"testing"
)

func TestParseNetDevFrom(t *testing.T) {
	input := `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
    lo: 6540586   61376    0    0    0     0          0         0  6540586   61376    0    0    0     0       0          0
  eth0:123456789  1000    1    2    0     0          0         0 987654321   2000    3    4    0     0       0          0
 wlan0: 5000 50    0    0    0     0          0         0     6000   60    0    0    0     0       0          0
`

	reader := strings.NewReader(input)
	results, err := parseNetDevFrom(reader)
	if err != nil {
		t.Fatalf("parseNetDevFrom failed: %v", err)
	}

	// Expect lo, eth0, wlan0
	if len(results) != 3 {
		fmt.Println(results)
		t.Errorf("Expected 3 interfaces, got %d", len(results))
	}

	eth, ok := results["eth0"]
	if !ok {
		t.Fatal("eth0 not found in results")
	}

	if eth.BytesRcvd != 123456789 {
		t.Errorf("eth0 RxBytes = %d, want 123456789", eth.BytesRcvd)
	}
	if eth.PacketsRcvd != 1000 {
		t.Errorf("eth0 RxPackets = %d, want 1000", eth.PacketsRcvd)
	}
	if eth.ErrorsRcvd != 1 {
		t.Errorf("eth0 RxErrors = %d, want 1", eth.ErrorsRcvd)
	}
	if eth.DropsRcvd != 2 {
		t.Errorf("eth0 RxDrops = %d, want 2", eth.DropsRcvd)
	}
	if eth.BytesSent != 987654321 {
		t.Errorf("eth0 TxBytes = %d, want 987654321", eth.BytesSent)
	}
	if eth.PacketsSent != 2000 {
		t.Errorf("eth0 TxPackets = %d, want 2000", eth.PacketsSent)
	}
	if eth.ErrorsSent != 3 {
		t.Errorf("eth0 TxErrors = %d, want 3", eth.ErrorsSent)
	}
	if eth.DropsSent != 4 {
		t.Errorf("eth0 TxDrops = %d, want 4", eth.DropsSent)
	}
}

func TestShouldIgnoreInterface(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"eth0", false},
		{"wlan0", false},
		{"enp3s0", false},
		{"lo", true},
		{"docker0", true},
		{"veth12345", true},
		{"br-08234", true},
		{"tun0", true},
		{"cali123", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldIgnoreInterface(tt.name)
			if got != tt.want {
				t.Errorf("shouldIgnoreInterface(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}
