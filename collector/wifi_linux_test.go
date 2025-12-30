//go:build !windows

package collector

import (
	"context"
	"strings"
	"testing"

	"github.com/nhdewitt/spectra/metrics"
)

func TestParseNetWirelessFrom(t *testing.T) {
	// Sample data from /proc/net/wireless
	// Format: interface: status link level noise ...
	input := `Inter-| sta-|   Quality        |   Discarded packets               | Missed | WE
 face | tus | link level noise |  nwid  crypt   frag  retry   misc | beacon | 22
  wlan0: 0000   60.  -50.  -256        0      0      0      0      0        0`

	// Define a mock fetcher that returns static data (SSID, Freq, BitRate)
	mockFetcher := func(ctx context.Context, iface string) (string, float64, float64) {
		if iface == "wlan0" {
			// Return SSID, Frequency (5.2 GHz), Bitrate (866.7 Mbps)
			return "TestNetwork", 5.2, 866.7
		}
		return "", 0.0, 0.0
	}

	results, err := parseNetWirelessFrom(context.Background(), strings.NewReader(input), mockFetcher)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	// Type assertion to access specific WiFi fields
	m, ok := results[0].(metrics.WiFiMetric)
	if !ok {
		t.Fatal("Result is not a WiFiMetric")
	}

	// Assertions
	if m.Interface != "wlan0" {
		t.Errorf("Expected interface wlan0, got %s", m.Interface)
	}
	if m.SignalLevel != -50 {
		t.Errorf("Expected signal -50, got %d", m.SignalLevel)
	}
	if m.LinkQuality != 60 {
		t.Errorf("Expected link quality 60, got %d", m.LinkQuality)
	}
	if m.SSID != "TestNetwork" {
		t.Errorf("Expected SSID TestNetwork, got %s", m.SSID)
	}
	if m.Frequency != 5.2 {
		t.Errorf("Expected frequency 5.2, got %f", m.Frequency)
	}
	if m.BitRate != 866.7 {
		t.Errorf("Expected bitrate 866.7, got %f", m.BitRate)
	}
}
