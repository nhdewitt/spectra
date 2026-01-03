//go:build !windows

package collector

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/nhdewitt/spectra/internal/protocol"
)

type metadataFetcher func(ctx context.Context, iface string) (string, float64, float64)

var (
	reSSID    = regexp.MustCompile(`SSID: (.+)`)
	reFreq    = regexp.MustCompile(`freq: (\d+)`)
	reBitRate = regexp.MustCompile(`tx bitrate: ([\d.]+)`)
)

func CollectWiFi(ctx context.Context) ([]protocol.Metric, error) {
	return parseNetWireless(ctx, getWiFiMetadata)
}

func parseNetWireless(ctx context.Context, fetcher metadataFetcher) ([]protocol.Metric, error) {
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

func parseNetWirelessFrom(ctx context.Context, r io.Reader, fetcher metadataFetcher) ([]protocol.Metric, error) {
	var results []protocol.Metric
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

		ssid, freq, bitrate := fetcher(ctx, iface)

		if ssid == "" {
			continue
		}

		metric := protocol.WiFiMetric{
			Interface:   iface,
			SignalLevel: int(sigLevel),
			LinkQuality: int(linkQual),
			SSID:        ssid,
			Frequency:   freq,
			BitRate:     bitrate,
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
func getWiFiMetadata(ctx context.Context, iface string) (ssid string, freq, bitrate float64) {
	// iw dev <interface> link
	out, err := exec.CommandContext(ctx, "iw", "dev", iface, "link").Output()
	if err != nil {
		return "", 0.0, 0.0
	}

	output := string(out)

	// Parse SSID
	if match := reSSID.FindStringSubmatch(output); len(match) > 1 {
		ssid = match[1]
	}

	// Parse frequency
	if match := reFreq.FindStringSubmatch(output); len(match) > 1 {
		val, err := strconv.ParseFloat(match[1], 64)
		if err != nil {
			return ssid, 0.0, 0.0
		}
		freq = val / 1000.0
	}

	// Parse Bitrate
	if match := reBitRate.FindStringSubmatch(output); len(match) > 1 {
		val, err := strconv.ParseFloat(match[1], 64)
		if err != nil {
			return ssid, freq, 0.0
		}
		bitrate = val
	}

	return ssid, freq, bitrate
}
