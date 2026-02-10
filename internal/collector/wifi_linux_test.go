//go:build linux

package collector

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/nhdewitt/spectra/internal/protocol"
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
	m, ok := results[0].(protocol.WiFiMetric)
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

func TestParseNetWirelessFrom_MultipleInterfaces(t *testing.T) {
	input := `Inter-| sta-|   Quality        |   Discarded packets               | Missed | WE
 face | tus | link level noise |  nwid  crypt   frag  retry   misc | beacon | 22
  wlan0: 0000   70.  -40.  -256        0      0      0      0      0        0
  wlan1: 0000   50.  -60.  -256        0      0      0      0      0        0`

	mockFetcher := func(ctx context.Context, iface string) (string, float64, float64) {
		switch iface {
		case "wlan0":
			return "Network1", 2.4, 150.0
		case "wlan1":
			return "Network2", 5.8, 433.0
		}
		return "", 0.0, 0.0
	}

	results, err := parseNetWirelessFrom(context.Background(), strings.NewReader(input), mockFetcher)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}

	m0 := results[0].(protocol.WiFiMetric)
	m1 := results[1].(protocol.WiFiMetric)

	if m0.Interface != "wlan0" || m0.SSID != "Network1" {
		t.Errorf("First interface mismatch: %+v", m0)
	}
	if m1.Interface != "wlan1" || m1.SSID != "Network2" {
		t.Errorf("Second interface mismatch: %+v", m1)
	}
}

func TestParseNetWirelessFrom_Empty(t *testing.T) {
	input := `Inter-| sta-|   Quality        |   Discarded packets               | Missed | WE
 face | tus | link level noise |  nwid  crypt   frag  retry   misc | beacon | 22`

	mockFetcher := func(ctx context.Context, iface string) (string, float64, float64) {
		return "Test", 5.0, 100.0
	}

	results, err := parseNetWirelessFrom(context.Background(), strings.NewReader(input), mockFetcher)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("Expected 0 results for header-only input, got %d", len(results))
	}
}

func TestParseNetWirelessFrom_NoSSID(t *testing.T) {
	input := `Inter-| sta-|   Quality        |   Discarded packets               | Missed | WE
 face | tus | link level noise |  nwid  crypt   frag  retry   misc | beacon | 22
  wlan0: 0000   60.  -50.  -256        0      0      0      0      0        0`

	// Mock returns empty SSID (not connected)
	mockFetcher := func(ctx context.Context, iface string) (string, float64, float64) {
		return "", 0.0, 0.0
	}

	results, err := parseNetWirelessFrom(context.Background(), strings.NewReader(input), mockFetcher)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should skip interfaces with no SSID
	if len(results) != 0 {
		t.Errorf("Expected 0 results when no SSID, got %d", len(results))
	}
}

func TestParseNetWirelessFrom_MalformedLine(t *testing.T) {
	input := `Inter-| sta-|   Quality        |   Discarded packets               | Missed | WE
 face | tus | link level noise |  nwid  crypt   frag  retry   misc | beacon | 22
  wlan0: 0000   60.`

	mockFetcher := func(ctx context.Context, iface string) (string, float64, float64) {
		return "Test", 5.0, 100.0
	}

	results, err := parseNetWirelessFrom(context.Background(), strings.NewReader(input), mockFetcher)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Malformed lines should be skipped
	if len(results) != 0 {
		t.Errorf("Expected 0 results for malformed input, got %d", len(results))
	}
}

func TestParseNetWirelessFrom_PositiveSignal(t *testing.T) {
	// Some drivers report positive signal levels
	input := `Inter-| sta-|   Quality        |   Discarded packets               | Missed | WE
 face | tus | link level noise |  nwid  crypt   frag  retry   misc | beacon | 22
  wlan0: 0000   70.  50.  -256        0      0      0      0      0        0`

	mockFetcher := func(ctx context.Context, iface string) (string, float64, float64) {
		return "TestNetwork", 5.2, 866.7
	}

	results, err := parseNetWirelessFrom(context.Background(), strings.NewReader(input), mockFetcher)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	m := results[0].(protocol.WiFiMetric)
	if m.SignalLevel != 50 {
		t.Errorf("Expected signal 50, got %d", m.SignalLevel)
	}
}

func TestParseNetWirelessFrom_WhitespaceVariations(t *testing.T) {
	// Different whitespace formatting
	input := `Inter-| sta-|   Quality        |   Discarded packets               | Missed | WE
 face | tus | link level noise |  nwid  crypt   frag  retry   misc | beacon | 22
wlan0:    0000    60.    -50.    -256        0      0      0      0      0        0`

	mockFetcher := func(ctx context.Context, iface string) (string, float64, float64) {
		return "TestNetwork", 5.2, 866.7
	}

	results, err := parseNetWirelessFrom(context.Background(), strings.NewReader(input), mockFetcher)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	m := results[0].(protocol.WiFiMetric)
	if m.Interface != "wlan0" {
		t.Errorf("Expected interface wlan0, got %s", m.Interface)
	}
}

func TestParseFloat(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    float64
		wantErr bool
	}{
		{"Integer", "60", 60.0, false},
		{"With Trailing Dot", "60.", 60.0, false},
		{"Negative", "-50", -50.0, false},
		{"Negative With Dot", "-50.", -50.0, false},
		{"Float", "60.5", 60.5, false},
		{"Zero", "0", 0.0, false},
		{"Invalid", "abc", 0.0, true},
		{"Empty", "", 0.0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseFloat(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("parseFloat(%q) = %f, want %f", tt.input, got, tt.want)
			}
		})
	}
}

func TestRegexPatterns(t *testing.T) {
	t.Run("SSID Pattern", func(t *testing.T) {
		tests := []struct {
			input string
			want  string
		}{
			{"SSID: MyNetwork", "MyNetwork"},
			{"SSID: Network With Spaces", "Network With Spaces"},
			{"SSID: ", ""},
			{"Connected to SSID: Test", "Test"},
		}

		for _, tt := range tests {
			match := reSSID.FindStringSubmatch(tt.input)
			got := ""
			if len(match) > 1 {
				got = match[1]
			}
			if got != tt.want {
				t.Errorf("reSSID.FindStringSubmatch(%q) = %q, want %q", tt.input, got, tt.want)
			}
		}
	})

	t.Run("Frequency Pattern", func(t *testing.T) {
		tests := []struct {
			input string
			want  string
		}{
			{"freq: 2437", "2437"},
			{"freq: 5180", "5180"},
			{"frequency: 2437", ""}, // Wrong prefix
		}

		for _, tt := range tests {
			match := reFreq.FindStringSubmatch(tt.input)
			got := ""
			if len(match) > 1 {
				got = match[1]
			}
			if got != tt.want {
				t.Errorf("reFreq.FindStringSubmatch(%q) = %q, want %q", tt.input, got, tt.want)
			}
		}
	})

	t.Run("BitRate Pattern", func(t *testing.T) {
		tests := []struct {
			input string
			want  string
		}{
			{"tx bitrate: 866.7", "866.7"},
			{"tx bitrate: 150", "150"},
			{"tx bitrate: 54.0 MBit/s", "54.0"},
			{"rx bitrate: 100", ""}, // Wrong prefix
		}

		for _, tt := range tests {
			match := reBitRate.FindStringSubmatch(tt.input)
			got := ""
			if len(match) > 1 {
				got = match[1]
			}
			if got != tt.want {
				t.Errorf("reBitRate.FindStringSubmatch(%q) = %q, want %q", tt.input, got, tt.want)
			}
		}
	})
}

func TestCollectWiFi_Integration(t *testing.T) {
	// Check if /proc/net/wireless exists
	if _, err := os.Stat("/proc/net/wireless"); os.IsNotExist(err) {
		t.Skip("/proc/net/wireless not available")
	}

	// Check if iw command exists
	if _, err := exec.LookPath("iw"); err != nil {
		t.Skip("iw command not available")
	}

	ctx := context.Background()
	metrics, err := CollectWiFi(ctx)
	if err != nil {
		t.Fatalf("CollectWiFi failed: %v", err)
	}

	t.Logf("Found %d WiFi interfaces", len(metrics))

	for _, m := range metrics {
		wifi, ok := m.(protocol.WiFiMetric)
		if !ok {
			t.Errorf("Expected WiFiMetric, got %T", m)
			continue
		}

		t.Logf("Interface: %s, SSID: %s, Signal: %d dBm, Quality: %d, Freq: %.1f GHz, BitRate: %.1f Mbps",
			wifi.Interface, wifi.SSID, wifi.SignalLevel, wifi.LinkQuality, wifi.Frequency, wifi.BitRate)

		// Sanity checks
		if wifi.SignalLevel > 0 || wifi.SignalLevel < -100 {
			t.Logf("Warning: Signal level %d dBm seems unusual", wifi.SignalLevel)
		}

		if wifi.LinkQuality < 0 || wifi.LinkQuality > 100 {
			t.Logf("Warning: Link quality %d seems unusual", wifi.LinkQuality)
		}

		if wifi.Frequency > 0 && (wifi.Frequency < 2.0 || wifi.Frequency > 7.0) {
			t.Logf("Warning: Frequency %.1f GHz seems unusual", wifi.Frequency)
		}
	}
}

func TestCollectWiFi_NoWirelessFile(t *testing.T) {
	// This tests the behavior when /proc/net/wireless doesn't exist
	// Can't easily test without mocking, but we can verify the function handles it
	ctx := context.Background()
	metrics, err := CollectWiFi(ctx)
	// Should return nil, nil if no wireless (not an error)
	if err != nil {
		t.Logf("CollectWiFi returned error: %v", err)
	}

	t.Logf("CollectWiFi returned %d metrics", len(metrics))
}

func TestCollectWiFi_ContextCancel(t *testing.T) {
	if _, err := os.Stat("/proc/net/wireless"); os.IsNotExist(err) {
		t.Skip("/proc/net/wireless not available")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Should handle cancelled context gracefully
	_, err := CollectWiFi(ctx)
	if err != nil {
		t.Logf("CollectWiFi with cancelled context: %v", err)
	}
}

func BenchmarkParseFloat(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_, _ = parseFloat("-50.")
	}
}

func BenchmarkParseFloat_NoDot(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_, _ = parseFloat("-50")
	}
}

func BenchmarkRegex_SSID(b *testing.B) {
	input := "SSID: MyTestNetwork"
	b.ReportAllocs()
	for b.Loop() {
		_ = reSSID.FindStringSubmatch(input)
	}
}

func BenchmarkRegex_Freq(b *testing.B) {
	input := "freq: 5180"
	b.ReportAllocs()
	for b.Loop() {
		_ = reFreq.FindStringSubmatch(input)
	}
}

func BenchmarkRegex_BitRate(b *testing.B) {
	input := "tx bitrate: 866.7 MBit/s"
	b.ReportAllocs()
	for b.Loop() {
		_ = reBitRate.FindStringSubmatch(input)
	}
}

func BenchmarkParseNetWirelessFrom(b *testing.B) {
	input := `Inter-| sta-|   Quality        |   Discarded packets               | Missed | WE
 face | tus | link level noise |  nwid  crypt   frag  retry   misc | beacon | 22
  wlan0: 0000   60.  -50.  -256        0      0      0      0      0        0`

	mockFetcher := func(ctx context.Context, iface string) (string, float64, float64) {
		return "TestNetwork", 5.2, 866.7
	}

	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		r := strings.NewReader(input)
		_, _ = parseNetWirelessFrom(ctx, r, mockFetcher)
	}
}

func BenchmarkParseNetWirelessFrom_MultipleInterfaces(b *testing.B) {
	input := `Inter-| sta-|   Quality        |   Discarded packets               | Missed | WE
 face | tus | link level noise |  nwid  crypt   frag  retry   misc | beacon | 22
  wlan0: 0000   70.  -40.  -256        0      0      0      0      0        0
  wlan1: 0000   50.  -60.  -256        0      0      0      0      0        0
  wlan2: 0000   60.  -50.  -256        0      0      0      0      0        0`

	mockFetcher := func(ctx context.Context, iface string) (string, float64, float64) {
		return "Network", 5.0, 100.0
	}

	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		r := strings.NewReader(input)
		_, _ = parseNetWirelessFrom(ctx, r, mockFetcher)
	}
}

func BenchmarkCollectWiFi(b *testing.B) {
	if _, err := os.Stat("/proc/net/wireless"); os.IsNotExist(err) {
		b.Skip("/proc/net/wireless not available")
	}

	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		_, _ = CollectWiFi(ctx)
	}
}
