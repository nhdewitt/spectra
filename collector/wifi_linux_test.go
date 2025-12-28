//go:build !windows

package collector

import (
	"context"
	"strings"
	"testing"

	"github.com/nhdewitt/raspimon/metrics"
)

func TestParseNetWirelessFrom(t *testing.T) {
	// Sample data from /proc/net/wireless
	input := `Inter-| sta-|   Quality        |   Discarded packets               | Missed | WE
 face | tus | link level noise |  nwid  crypt   frag  retry   misc | beacon | 22
  wlan0: 0000   60.  -50.  -256        0      0      0      0      0        0`

	// Define a mock fetcher that returns static data
	mockFetcher := func(ctx context.Context, iface string) (string, float64) {
		if iface == "wlan0" {
			return "TestNetwork", 5.2
		}
		return "", 0.0
	}

	results, err := parseNetWirelessFrom(context.Background(), strings.NewReader(input), mockFetcher)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	m, ok := results[0].(metrics.WiFiMetric)
	if !ok {
		t.Fatal("Result is not a WiFiMetric")
	}

	if m.Interface != "wlan0" {
		t.Errorf("Expected interface wlan0, got %s", m.Interface)
	}
	if m.SignalLevel != -50 {
		t.Errorf("Expected signal -50, got %d", m.SignalLevel)
	}
	if m.SSID != "TestNetwork" {
		t.Errorf("Expected SSID TestNetwork, got %s", m.SSID)
	}
}
