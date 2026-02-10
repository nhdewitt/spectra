//go:build linux

package collector

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
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

	if eth.RxBytes != 123456789 {
		t.Errorf("eth0 RxBytes = %d, want 123456789", eth.RxBytes)
	}
	if eth.RxPackets != 1000 {
		t.Errorf("eth0 RxPackets = %d, want 1000", eth.RxPackets)
	}
	if eth.RxErrors != 1 {
		t.Errorf("eth0 RxErrors = %d, want 1", eth.RxErrors)
	}
	if eth.RxDrops != 2 {
		t.Errorf("eth0 RxDrops = %d, want 2", eth.RxDrops)
	}
	if eth.TxBytes != 987654321 {
		t.Errorf("eth0 TxBytes = %d, want 987654321", eth.TxBytes)
	}
	if eth.TxPackets != 2000 {
		t.Errorf("eth0 TxPackets = %d, want 2000", eth.TxPackets)
	}
	if eth.TxErrors != 3 {
		t.Errorf("eth0 TxErrors = %d, want 3", eth.TxErrors)
	}
	if eth.TxDrops != 4 {
		t.Errorf("eth0 TxDrops = %d, want 4", eth.TxDrops)
	}
}

func TestParseNetDevFrom_Empty(t *testing.T) {
	input := `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
`
	reader := strings.NewReader(input)
	results, err := parseNetDevFrom(reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 interfaces, got %d", len(results))
	}
}

func TestParseNetDevFrom_MalformedLines(t *testing.T) {
	input := `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
    lo: 100 200
  eth0:123456789  1000    1    2    0     0          0         0 987654321   2000    3    4    0     0       0          0
short
`
	reader := strings.NewReader(input)
	results, err := parseNetDevFrom(reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 interface, got %d", len(results))
	}
	if _, ok := results["eth0"]; !ok {
		t.Error("eth0 not found")
	}
}

func TestParseNetDevFrom_NoSpaceAfterColon(t *testing.T) {
	input := `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
eth0:123456789  1000    1    2    0     0          0         0 987654321   2000    3    4    0     0       0          0
`
	reader := strings.NewReader(input)
	results, err := parseNetDevFrom(reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := results["eth0"]; !ok {
		t.Error("eth0 not found - colon parsing may have failed")
	}
}

func TestParseNetDevFrom_LargeValues(t *testing.T) {
	input := `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
  eth0:18446744073709551615  1000    0    0    0     0          0         0 18446744073709551615   2000    0    0    0     0       0          0
`
	reader := strings.NewReader(input)
	results, err := parseNetDevFrom(reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	eth := results["eth0"]
	if eth.RxBytes != 18446744073709551615 {
		t.Errorf("RxBytes = %d, want max uint64", eth.RxBytes)
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

func TestShouldIgnoreInterface_AllPrefixes(t *testing.T) {
	for _, prefix := range ignoredInterfacePrefixes {
		name := prefix + "0"
		if !shouldIgnoreInterface(name) {
			t.Errorf("shouldIgnoreInterface(%q) + false, want true", name)
		}
	}
}

func TestShouldIgnoreInterface_CommonPhysical(t *testing.T) {
	physical := []string{
		"eth0", "eth1", "enp0s3", "enp3s0", "eno1",
		"wlan0", "wlp2s0", "ens192", "ens33",
	}
	for _, name := range physical {
		if shouldIgnoreInterface(name) {
			t.Errorf("shouldIgnoreInterface(%q) = true, want false", name)
		}
	}
}

func TestCollectNetwork_Baseline(t *testing.T) {
	lastNetworkRaw = nil
	lastNetworkTime = time.Time{}

	ctx := context.Background()

	metrics, err := CollectNetwork(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if metrics != nil {
		t.Error("expected nil metrics on baseline")
	}
	if lastNetworkRaw == nil {
		t.Error("lastNetworkRaw should be populated")
	}
}

func TestCollectNetwork_SecondCall(t *testing.T) {
	lastNetworkRaw = nil
	lastNetworkTime = time.Time{}

	ctx := context.Background()

	// Baseline
	_, _ = CollectNetwork(ctx)
	time.Sleep(10 * time.Millisecond)

	metrics, err := CollectNetwork(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Logf("Returned %d network metrics", len(metrics))

	for _, m := range metrics {
		nm := m.(protocol.NetworkMetric)
		t.Logf("  %s: RX=%d TX=%d", nm.Interface, nm.RxBytes, nm.TxBytes)
	}
}

func TestCollectNetwork_ZeroElapsed(t *testing.T) {
	lastNetworkRaw = map[string]NetworkRaw{
		"eth0": {Interface: "eth0", RxBytes: 100},
	}
	lastNetworkTime = time.Now().Add(1 * time.Second)

	ctx := context.Background()
	metrics, err := CollectNetwork(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if metrics != nil {
		t.Error("expected nil metrics when elapsed <= 0")
	}
	if lastNetworkRaw != nil {
		t.Error("lastNetworkRaw should be reset to nil")
	}
}

func TestGetLinuxMAC_Invalid(t *testing.T) {
	mac := getLinuxMAC("nonexistent_interface_12345")
	if mac != "" {
		t.Errorf("expected empty MAC for invalid interface, got %q", mac)
	}
}

func TestGetLinuxMTU_Invalid(t *testing.T) {
	mtu := getLinuxMTU("nonexistent_interface_12345")
	if mtu != 0 {
		t.Errorf("expected 0 MTU for invalid interface, got %d", mtu)
	}
}

func TestGetLinuxLinkSpeed_Invalid(t *testing.T) {
	speed := getLinuxLinkSpeed("nonexistent_interface_12345")
	if speed != 0 {
		t.Errorf("expected 0 speed for invalid interface, got %d", speed)
	}
}

func BenchmarkParseNetDevFrom(b *testing.B) {
	input := `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
    lo: 6540586   61376    0    0    0     0          0         0  6540586   61376    0    0    0     0       0          0
  eth0:123456789012  10000000    100    200    0     0          0         0 987654321098   20000000    300    400    0     0       0          0
 wlan0: 5000000000 5000000    0    0    0     0          0         0     6000000000   6000000    0    0    0     0       0          0
docker0: 1000 10    0    0    0     0          0         0     2000   20    0    0    0     0       0          0
  br-abc: 500 5    0    0    0     0          0         0     600   6    0    0    0     0       0          0
`
	b.ReportAllocs()
	for b.Loop() {
		r := strings.NewReader(input)
		_, _ = parseNetDevFrom(r)
	}
}

func BenchmarkShouldIgnoreInterface_Match(b *testing.B) {
	for b.Loop() {
		_ = shouldIgnoreInterface("docker0")
	}
}

func BenchmarkShouldIgnoreInterface_NoMatch(b *testing.B) {
	for b.Loop() {
		_ = shouldIgnoreInterface("eth0")
	}
}

func BenchmarkCollectNetwork(b *testing.B) {
	lastNetworkRaw = nil
	ctx := context.Background()
	_, _ = CollectNetwork(ctx)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = CollectNetwork(ctx)
	}
}
