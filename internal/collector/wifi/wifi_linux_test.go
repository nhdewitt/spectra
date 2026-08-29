//go:build linux

package wifi

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/nhdewitt/spectra/internal/protocol"
)

const associatedLink = `Connected to a4:2b:8c:11:22:33 (on wlan0)
	SSID: HomeNetwork
	freq: 5180
	RX: 981234 bytes (4321 packets)
	TX: 123456 bytes (987 packets)
	signal: -47 dBm
	tx bitrate: 866.7 MBit/s VHT-MCS 9 80MHz short GI VHT-NSS 2
`

func TestParseNetWirelessFrom(t *testing.T) {
	// Sample data from /proc/net/wireless
	// Format: interface: status link level noise ...
	input := `Inter-| sta-|   Quality        |   Discarded packets               | Missed | WE
 face | tus | link level noise |  nwid  crypt   frag  retry   misc | beacon | 22
  wlan0: 0000   60.  -50.  -256        0      0      0      0      0        0`

	// Define a mock fetcher that returns static data (SSID, Freq, BitRate)
	mockFetcher := func(ctx context.Context, iface string) (wifiMeta, error) {
		if iface == "wlan0" {
			// Return SSID, Frequency (5.2 GHz), Bitrate (866.7 Mbps)
			return wifiMeta{SSID: "TestNetwork", Freq: 5.2, BitRate: 866.7}, nil
		}
		return wifiMeta{}, nil
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

	mockFetcher := func(ctx context.Context, iface string) (wifiMeta, error) {
		switch iface {
		case "wlan0":
			return wifiMeta{SSID: "Network1", Freq: 2.4, BitRate: 150.0}, nil
		case "wlan1":
			return wifiMeta{SSID: "Network2", Freq: 5.8, BitRate: 433.0}, nil
		}
		return wifiMeta{}, nil
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

	mockFetcher := func(ctx context.Context, iface string) (wifiMeta, error) {
		return wifiMeta{SSID: "Test", Freq: 5.0, BitRate: 100.0}, nil
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
	mockFetcher := func(ctx context.Context, iface string) (wifiMeta, error) {
		return wifiMeta{}, nil
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

	mockFetcher := func(ctx context.Context, iface string) (wifiMeta, error) {
		return wifiMeta{SSID: "Test", Freq: 5.0, BitRate: 100.0}, nil
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

	mockFetcher := func(ctx context.Context, iface string) (wifiMeta, error) {
		return wifiMeta{SSID: "TestNetwork", Freq: 5.2, BitRate: 866.7}, nil
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

	mockFetcher := func(ctx context.Context, iface string) (wifiMeta, error) {
		return wifiMeta{SSID: "TestNetwork", Freq: 5.2, BitRate: 866.7}, nil
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

func TestCollect_Integration(t *testing.T) {
	// Check if /proc/net/wireless exists
	if _, err := os.Stat("/proc/net/wireless"); os.IsNotExist(err) {
		t.Skip("/proc/net/wireless not available")
	}

	// Check if iw command exists
	if _, err := exec.LookPath("iw"); err != nil {
		t.Skip("iw command not available")
	}

	ctx := context.Background()
	metrics, err := Collect(ctx)
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
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

func TestCollect_NoWirelessFile(t *testing.T) {
	// This tests the behavior when /proc/net/wireless doesn't exist
	// Can't easily test without mocking, but we can verify the function handles it
	ctx := context.Background()
	metrics, err := Collect(ctx)
	// Should return nil, nil if no wireless (not an error)
	if err != nil {
		t.Logf("Collect returned error: %v", err)
	}

	t.Logf("Collect returned %d metrics", len(metrics))
}

func TestCollect_ContextCancel(t *testing.T) {
	if _, err := os.Stat("/proc/net/wireless"); os.IsNotExist(err) {
		t.Skip("/proc/net/wireless not available")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Should handle cancelled context gracefully
	_, err := Collect(ctx)
	if err != nil {
		t.Logf("Collect with cancelled context: %v", err)
	}
}

func TestParseIWLink_Associated(t *testing.T) {
	meta := parseIWLink(associatedLink)

	if !meta.Associated() {
		t.Fatal("expected Associated() to be true")
	}
	if meta.SSID != "HomeNetwork" {
		t.Errorf("SSID = %q, want %q", meta.SSID, "HomeNetwork")
	}
	if meta.Freq != 5.18 {
		t.Errorf("Freq = %v, want 5.18 (GHz, converted from MHz)", meta.Freq)
	}
	if meta.BitRate != 866.7 {
		t.Errorf("BitRate = %v, want 866.7", meta.BitRate)
	}
}

// An idle interface is not a failure. iw exits zero and prints this, so the
// zero wifiMeta with no error is the correct result.
func TestParseIWLink_NotAssociated(t *testing.T) {
	meta := parseIWLink("Not connected.\n")

	if meta.Associated() {
		t.Error("expected Associated() to be false")
	}
	if meta != (wifiMeta{}) {
		t.Errorf("expected the zero value, got %+v", meta)
	}
}

// iw omits tx bitrate briefly after association. Losing the SSID and
// frequency over a missing rate would drop the interface entirely.
func TestParseIWLink_MissingBitRate(t *testing.T) {
	out := "Connected to a4:2b:8c:11:22:33 (on wlan0)\n\tSSID: HomeNetwork\n\tfreq: 2412\n"

	meta := parseIWLink(out)

	if meta.SSID != "HomeNetwork" {
		t.Errorf("SSID = %q, want %q", meta.SSID, "HomeNetwork")
	}
	if meta.Freq != 2.412 {
		t.Errorf("Freq = %v, want 2.412", meta.Freq)
	}
	if meta.BitRate != 0 {
		t.Errorf("BitRate = %v, want 0", meta.BitRate)
	}
}

// reBitRate is ([\d.]+), which matches "1.2.3". A malformed rate must not
// discard the fields that parsed cleanly.
func TestParseIWLink_MalformedBitRateKeepsOtherFields(t *testing.T) {
	out := "\tSSID: HomeNetwork\n\tfreq: 2412\n\ttx bitrate: 1.2.3 MBit/s\n"

	meta := parseIWLink(out)

	if meta.SSID != "HomeNetwork" {
		t.Errorf("SSID = %q, want %q", meta.SSID, "HomeNetwork")
	}
	if meta.Freq != 2.412 {
		t.Errorf("Freq = %v, want 2.412", meta.Freq)
	}
	if meta.BitRate != 0 {
		t.Errorf("BitRate = %v, want 0", meta.BitRate)
	}
}

func TestParseIWLink_SSIDWithSpaces(t *testing.T) {
	meta := parseIWLink("\tSSID: My Home Network\n\tfreq: 2412\n")

	if meta.SSID != "My Home Network" {
		t.Errorf("SSID = %q, want %q", meta.SSID, "My Home Network")
	}
}

func TestParseIWLink_Empty(t *testing.T) {
	if meta := parseIWLink(""); meta != (wifiMeta{}) {
		t.Errorf("expected the zero value, got %+v", meta)
	}
}

// The point of the signature change: a fetch failure is reported instead of
// looking identical to an idle interface.
func TestParseNetWirelessFrom_FetchErrorIsReported(t *testing.T) {
	input := `Inter-| sta-|   Quality        |   Discarded packets               | Missed | WE
 face | tus | link level noise |  nwid  crypt   frag  retry   misc | beacon | 22
 wlan0: 0000   70.  -40.  -256        0      0      0      0      0        0
`
	failing := func(ctx context.Context, iface string) (wifiMeta, error) {
		return wifiMeta{}, exec.ErrNotFound
	}

	results, err := parseNetWirelessFrom(context.Background(), strings.NewReader(input), failing)

	if err == nil {
		t.Fatal("expected an error when the fetcher fails, got nil")
	}
	if !errors.Is(err, exec.ErrNotFound) {
		t.Errorf("expected exec.ErrNotFound in the chain, got %v", err)
	}
	if !strings.Contains(err.Error(), "wlan0") {
		t.Errorf("error does not name the interface: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected no metrics, got %v", results)
	}
}

// An idle interface is silently skipped, with no error. This is the half of
// the split that must stay quiet.
func TestParseNetWirelessFrom_IdleInterfaceIsSilent(t *testing.T) {
	input := `Inter-| sta-|   Quality        |   Discarded packets               | Missed | WE
 face | tus | link level noise |  nwid  crypt   frag  retry   misc | beacon | 22
 wlan0: 0000   70.  -40.  -256        0      0      0      0      0        0
`
	idle := func(ctx context.Context, iface string) (wifiMeta, error) {
		return wifiMeta{}, nil
	}

	results, err := parseNetWirelessFrom(context.Background(), strings.NewReader(input), idle)
	if err != nil {
		t.Errorf("an idle interface must not produce an error, got %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected no metrics, got %v", results)
	}
}

// One interface failing must not cost the others.
func TestParseNetWirelessFrom_PartialFailureStillReports(t *testing.T) {
	input := `Inter-| sta-|   Quality        |   Discarded packets               | Missed | WE
 face | tus | link level noise |  nwid  crypt   frag  retry   misc | beacon | 22
 wlan0: 0000   70.  -40.  -256        0      0      0      0      0        0
 wlan1: 0000   65.  -55.  -256        0      0      0      0      0        0
`
	mixed := func(ctx context.Context, iface string) (wifiMeta, error) {
		if iface == "wlan0" {
			return wifiMeta{}, errors.New("iw exploded")
		}
		return wifiMeta{SSID: "Second", Freq: 2.412, BitRate: 100}, nil
	}

	results, err := parseNetWirelessFrom(context.Background(), strings.NewReader(input), mixed)

	if err == nil {
		t.Error("expected the wlan0 failure to be reported")
	}
	if len(results) != 1 {
		t.Fatalf("expected wlan1 to still report, got %d metrics", len(results))
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

	mockFetcher := func(ctx context.Context, iface string) (wifiMeta, error) {
		return wifiMeta{SSID: "TestNetwork", Freq: 5.2, BitRate: 866.7}, nil
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

	mockFetcher := func(ctx context.Context, iface string) (wifiMeta, error) {
		return wifiMeta{SSID: "Network", Freq: 5.0, BitRate: 100.0}, nil
	}

	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		r := strings.NewReader(input)
		_, _ = parseNetWirelessFrom(ctx, r, mockFetcher)
	}
}

func BenchmarkCollect(b *testing.B) {
	if _, err := os.Stat("/proc/net/wireless"); os.IsNotExist(err) {
		b.Skip("/proc/net/wireless not available")
	}

	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		_, _ = Collect(ctx)
	}
}
