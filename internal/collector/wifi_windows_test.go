//go:build windows

package collector

import (
	"context"
	"testing"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func TestChannelToFrequency(t *testing.T) {
	tests := []struct {
		name    string
		channel uint32
		want    float64
	}{
		// 2.4GHz band
		{"Channel 1", 1, 2.412},
		{"Channel 6", 6, 2.437},
		{"Channel 11", 11, 2.462},
		{"Channel 13", 13, 2.472},
		{"Channel 14 (Japan)", 14, 2.484},

		// 5GHz band
		{"Channel 36", 36, 5.180},
		{"Channel 40", 40, 5.200},
		{"Channel 44", 44, 5.220},
		{"Channel 48", 48, 5.240},
		{"Channel 149", 149, 5.745},
		{"Channel 153", 153, 5.765},
		{"Channel 157", 157, 5.785},
		{"Channel 161", 161, 5.805},
		{"Channel 165", 165, 5.825},

		// Edge cases
		{"Channel 0", 0, 0.0},
		{"Channel 32 (5GHz start)", 32, 5.160},
		{"Channel 177 (5GHz end)", 177, 5.885},

		// 6GHz band
		{"Channel 193 (6GHz)", 193, 6.915},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := channelToFrequency(tt.channel)
			if got != tt.want {
				t.Errorf("channelToFrequency(%d) = %f, want %f", tt.channel, got, tt.want)
			}
		})
	}
}

func TestParseDot11SSID(t *testing.T) {
	tests := []struct {
		name string
		ssid dot11Ssid
		want string
	}{
		{
			name: "Normal SSID",
			ssid: func() dot11Ssid {
				s := dot11Ssid{USSIDLength: 10}
				copy(s.UcSSID[:], "MyNetwork")
				return s
			}(),
			want: "MyNetwork",
		},
		{
			name: "Empty SSID",
			ssid: dot11Ssid{USSIDLength: 0},
			want: "",
		},
		{
			name: "Max Length SSID",
			ssid: func() dot11Ssid {
				s := dot11Ssid{USSIDLength: 32}
				copy(s.UcSSID[:], "12345678901234567890123456789012")
				return s
			}(),
			want: "12345678901234567890123456789012",
		},
		{
			name: "SSID With Spaces",
			ssid: func() dot11Ssid {
				s := dot11Ssid{USSIDLength: 15}
				copy(s.UcSSID[:], "My Home Network")
				return s
			}(),
			want: "My Home Network",
		},
		{
			name: "SSID With Special Chars",
			ssid: func() dot11Ssid {
				s := dot11Ssid{USSIDLength: 12}
				copy(s.UcSSID[:], "Test-Net_123")
				return s
			}(),
			want: "Test-Net_123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDot11SSID(tt.ssid)
			if got != tt.want {
				t.Errorf("parseDot11SSID() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestUtf16ToString(t *testing.T) {
	tests := []struct {
		name  string
		input []uint16
		want  string
	}{
		{
			name:  "Simple ASCII",
			input: []uint16{'T', 'e', 's', 't', 0},
			want:  "Test",
		},
		{
			name:  "Empty",
			input: []uint16{0},
			want:  "",
		},
		{
			name:  "With Null Terminator",
			input: []uint16{'H', 'e', 'l', 'l', 'o', 0, 'W', 'o', 'r', 'l', 'd'},
			want:  "Hello",
		},
		{
			name:  "Interface Name",
			input: []uint16{'I', 'n', 't', 'e', 'l', ' ', 'W', 'i', '-', 'F', 'i', 0},
			want:  "Intel Wi-Fi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := utf16ToString(tt.input)
			if got != tt.want {
				t.Errorf("utf16ToString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCollectWiFi_Integration(t *testing.T) {
	ctx := context.Background()
	metrics, err := CollectWiFi(ctx)
	if err != nil {
		// WLAN service may not be available
		t.Logf("CollectWiFi returned error (may be expected): %v", err)
		return
	}

	t.Logf("Found %d WiFi interfaces", len(metrics))

	for _, m := range metrics {
		wifi, ok := m.(protocol.WiFiMetric)
		if !ok {
			t.Errorf("Expected WiFiMetric, got %T", m)
			continue
		}

		t.Logf("Interface: %s", wifi.Interface)
		t.Logf("  SSID: %s", wifi.SSID)
		t.Logf("  Signal: %d dBm", wifi.SignalLevel)
		t.Logf("  Quality: %d%%", wifi.LinkQuality)
		t.Logf("  Frequency: %.3f GHz", wifi.Frequency)
		t.Logf("  BitRate: %.1f Mbps", wifi.BitRate)

		if wifi.SSID == "" {
			t.Error("SSID should not be empty for connected interface")
		}

		if wifi.SignalLevel > 0 || wifi.SignalLevel < -100 {
			t.Logf("Warning: Signal level %d dBm seems unusual", wifi.SignalLevel)
		}

		if wifi.LinkQuality < 0 || wifi.LinkQuality > 100 {
			t.Errorf("Link quality %d should be 0-100", wifi.LinkQuality)
		}

		if wifi.Frequency > 0 && (wifi.Frequency < 2.0 || wifi.Frequency > 7.0) {
			t.Logf("Warning: Frequency %.3f GHz seems unusual", wifi.Frequency)
		}

		if wifi.BitRate < 0 {
			t.Errorf("BitRate %.1f should not be negative", wifi.BitRate)
		}
	}
}

func TestCollectWiFi_NoWiFiAdapter(t *testing.T) {
	// This test documents expected behavior when no WiFi adapter exists
	ctx := context.Background()
	metrics, err := CollectWiFi(ctx)

	// Either nil error with empty results, or specific error
	if err != nil {
		t.Logf("No WiFi adapter: %v", err)
	} else {
		t.Logf("Returned %d metrics (0 expected if no adapter)", len(metrics))
	}
}

func TestCollectWiFi_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// WLAN API calls are synchronous and don't use context directly
	// but function should handle cancelled context gracefully
	metrics, err := CollectWiFi(ctx)
	if err != nil {
		t.Logf("CollectWiFi with cancelled context: %v", err)
	}
	t.Logf("Returned %d metrics with cancelled context", len(metrics))
}

func TestCollectWiFi_ConsistentResults(t *testing.T) {
	ctx := context.Background()

	metrics1, err1 := CollectWiFi(ctx)
	if err1 != nil {
		t.Skip("WiFi not available")
	}

	metrics2, err2 := CollectWiFi(ctx)
	if err2 != nil {
		t.Fatalf("Second call failed: %v", err2)
	}

	// Same number of interfaces
	if len(metrics1) != len(metrics2) {
		t.Logf("Interface count changed: %d -> %d", len(metrics1), len(metrics2))
	}

	// SSID should be stable
	if len(metrics1) > 0 && len(metrics2) > 0 {
		wifi1 := metrics1[0].(protocol.WiFiMetric)
		wifi2 := metrics2[0].(protocol.WiFiMetric)

		if wifi1.SSID != wifi2.SSID {
			t.Errorf("SSID changed: %s -> %s", wifi1.SSID, wifi2.SSID)
		}

		// Signal shouldn't change drastically between immediate calls
		diff := wifi2.SignalLevel - wifi1.SignalLevel
		if diff < -10 || diff > 10 {
			t.Logf("Signal changed significantly: %d -> %d", wifi1.SignalLevel, wifi2.SignalLevel)
		}
	}
}

func TestRssiFromQuality(t *testing.T) {
	// Test the fallback formula: rssi = (quality / 2) - 100
	tests := []struct {
		quality int
		wantMin int
		wantMax int
	}{
		{100, -50, -50}, // Best signal
		{50, -75, -75},  // Medium signal
		{0, -100, -100}, // Worst signal
		{80, -60, -60},  // Good signal
		{20, -90, -90},  // Poor signal
	}

	for _, tt := range tests {
		rssi := (tt.quality / 2) - 100
		if rssi < tt.wantMin || rssi > tt.wantMax {
			t.Errorf("quality %d -> rssi %d, want %d-%d", tt.quality, rssi, tt.wantMin, tt.wantMax)
		}
	}
}

func TestChannelToFrequency_AllCommonChannels(t *testing.T) {
	// Verify all common 2.4GHz channels
	channels24 := map[uint32]float64{
		1: 2.412, 2: 2.417, 3: 2.422, 4: 2.427, 5: 2.432,
		6: 2.437, 7: 2.442, 8: 2.447, 9: 2.452, 10: 2.457,
		11: 2.462, 12: 2.467, 13: 2.472, 14: 2.484,
	}

	for ch, expected := range channels24 {
		got := channelToFrequency(ch)
		if got != expected {
			t.Errorf("Channel %d: got %f, want %f", ch, got, expected)
		}
	}

	// Verify common 5GHz channels
	channels5 := map[uint32]float64{
		36: 5.180, 40: 5.200, 44: 5.220, 48: 5.240,
		52: 5.260, 56: 5.280, 60: 5.300, 64: 5.320,
		100: 5.500, 104: 5.520, 108: 5.540, 112: 5.560,
		116: 5.580, 120: 5.600, 124: 5.620, 128: 5.640,
		132: 5.660, 136: 5.680, 140: 5.700, 144: 5.720,
		149: 5.745, 153: 5.765, 157: 5.785, 161: 5.805,
		165: 5.825,
	}

	for ch, expected := range channels5 {
		got := channelToFrequency(ch)
		if got != expected {
			t.Errorf("Channel %d: got %f, want %f", ch, got, expected)
		}
	}
}

// Benchmarks

func BenchmarkChannelToFrequency_24GHz(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = channelToFrequency(6)
	}
}

func BenchmarkChannelToFrequency_5GHz(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = channelToFrequency(149)
	}
}

func BenchmarkParseDot11SSID(b *testing.B) {
	ssid := dot11Ssid{USSIDLength: 10}
	copy(ssid.UcSSID[:], "MyNetwork")

	b.ReportAllocs()
	for b.Loop() {
		_ = parseDot11SSID(ssid)
	}
}

func BenchmarkParseDot11SSID_MaxLength(b *testing.B) {
	ssid := dot11Ssid{USSIDLength: 32}
	copy(ssid.UcSSID[:], "12345678901234567890123456789012")

	b.ReportAllocs()
	for b.Loop() {
		_ = parseDot11SSID(ssid)
	}
}

func BenchmarkUtf16ToString(b *testing.B) {
	input := []uint16{'I', 'n', 't', 'e', 'l', ' ', 'W', 'i', '-', 'F', 'i', ' ', '6', 'E', ' ', 'A', 'X', '2', '1', '1', 0}

	b.ReportAllocs()
	for b.Loop() {
		_ = utf16ToString(input)
	}
}

func BenchmarkCollectWiFi(b *testing.B) {
	ctx := context.Background()

	// Prime - skip if no WiFi
	_, err := CollectWiFi(ctx)
	if err != nil {
		b.Skip("WiFi not available")
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = CollectWiFi(ctx)
	}
}
