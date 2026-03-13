//go:build darwin

package network

import (
	"context"
	"testing"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func TestCollectNetworkRaw(t *testing.T) {
	raw, err := collectNetworkRaw()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(raw) == 0 {
		t.Fatal("expected to find at least one interface")
	}

	var foundEn0 bool
	for name, r := range raw {
		t.Logf("%s: MAC=%s MTU=%d Speed=%d RxBytes=%d TxBytes=%d RxPkts=%d TxPkts=%d",
			name, r.MAC, r.MTU, r.Speed, r.RxBytes, r.TxBytes, r.RxPackets, r.TxPackets)
		if name == "en0" {
			foundEn0 = true
		}
	}

	if !foundEn0 {
		t.Log("en0 not found (might be expected)")
	}
}

func TestCollect_FirstSampleNil(t *testing.T) {
	lastNetworkRaw = nil
	lastNetworkTime = time.Time{}

	ctx := context.Background()
	metrics, err := Collect(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if metrics != nil {
		t.Errorf("expected nil on first sample, got %d metrics", len(metrics))
	}
}

func TestCollect_SecondSample(t *testing.T) {
	lastNetworkRaw = nil
	lastNetworkTime = time.Time{}

	ctx := context.Background()

	_, err := Collect(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	time.Sleep(1 * time.Second)

	metrics, err := Collect(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if metrics == nil {
		t.Fatal("second sample returned nil, expected metrics")
	}

	for _, m := range metrics {
		nm := m.(protocol.NetworkMetric)
		t.Logf("%s: rx=%d B/s tx=%d B/s rxpkt=%d/s txpkt=%d/s", nm.Interface, nm.RxBytes, nm.TxBytes, nm.RxPackets, nm.TxPackets)
	}
}

func TestCollectNetworkRaw_HasTraffic(t *testing.T) {
	raw, err := collectNetworkRaw()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var hasTraffic bool
	for name, r := range raw {
		if r.RxBytes > 0 || r.TxBytes > 0 {
			hasTraffic = true
			t.Logf("%s: RxBytes=%d TxBytes=%d", name, r.RxBytes, r.TxBytes)
		}
	}

	if !hasTraffic {
		t.Error("no traffic seen on any non-ignored interface")
	}
}

func BenchmarkCollectNetworkRaw(b *testing.B) {
	for b.Loop() {
		_, _ = collectNetworkRaw()
	}
}
