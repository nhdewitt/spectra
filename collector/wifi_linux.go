//go:build !windows

package collector

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/nhdewitt/spectra/metrics"
)

type metadataFetcher func(ctx context.Context, iface string) (string, float64)

func CollectWiFi(ctx context.Context) ([]metrics.Metric, error) {
	return parseNetWireless(ctx, getWiFiMetadata)
}

func parseNetWireless(ctx context.Context, fetcher metadataFetcher) ([]metrics.Metric, error) {
	f, err := os.Open("/proc/net/wireless")
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No Wi-Fi
		}
		return nil, fmt.Errorf("parsing /proc/net/wireless: %w", err)
	}
	defer f.Close()

	return parseNetWirelessFrom(ctx, f, fetcher)
}

func parseNetWirelessFrom(ctx context.Context, r io.Reader, fetcher metadataFetcher) ([]metrics.Metric, error) {
	var results []metrics.Metric
	scanner := bufio.NewScanner(r)

	for range 2 {
		if !scanner.Scan() {
			return results, scanner.Err()
		}
	}

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 4 {
			continue
		}

		// Field 0: "wlan0:" -> "wlan0"
		iface := strings.TrimSuffix(fields[0], ":")

		// Field 2: Link Quality
		linkQual, err := parseFloat(fields[2])
		if err != nil {
			return nil, err
		}

		// Field 3: Signal Level (dBm)
		sigLevel, err := parseFloat(fields[3])
		if err != nil {
			return nil, err
		}

		ssid, freq := fetcher(ctx, iface)

		metric := metrics.WiFiMetric{
			Interface:   iface,
			SignalLevel: int(sigLevel),
			LinkQuality: int(linkQual),
			SSID:        ssid,
			Frequency:   freq,
		}

		results = append(results, metric)
	}

	return results, scanner.Err()
}

// parseFloat strips trailing dots before parsing
func parseFloat(s string) (float64, error) {
	return strconv.ParseFloat(strings.TrimSuffix(s, "."), 64)
}

// getWiFiMetadata calls `iwgetid` to fetch SSID and Frequency
func getWiFiMetadata(ctx context.Context, iface string) (string, float64) {
	// iwgetid -r (SSID)
	out, _ := exec.CommandContext(ctx, "iwgetid", "-r", iface).Output()
	ssid := strings.TrimSpace(string(out))

	// iwgetid -f (Frequency)
	out, _ = exec.CommandContext(ctx, "iwgetid", "-f", iface).Output()
	freqStr := string(out)

	var freq float64
	if parts := strings.Split(freqStr, ":"); len(parts) > 1 {
		val := strings.Fields(parts[1])[0]
		freq, _ = strconv.ParseFloat(val, 64)
	}

	return ssid, freq
}
