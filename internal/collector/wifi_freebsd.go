//go:build freebsd

package collector

import (
	"bufio"
	"context"
	"net"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/nhdewitt/spectra/internal/protocol"
)

var (
	reSSID = regexp.MustCompile(`ssid\s+"([^"]+)"`)
	reChan = regexp.MustCompile(`channel\s+\d+\s+\((\d+)\s+MHz`)
)

func CollectWiFi(ctx context.Context) ([]protocol.Metric, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	var results []protocol.Metric
	for _, iface := range ifaces {
		if !strings.HasPrefix(iface.Name, "wlan") {
			continue
		}

		metric, err := collectWlanInterface(ctx, iface.Name)
		if err != nil || metric == nil {
			continue
		}
		results = append(results, metric)
	}

	if len(results) == 0 {
		return nil, nil
	}
	return results, nil
}

func collectWlanInterface(ctx context.Context, iface string) (protocol.Metric, error) {
	// ifconfig <iface> to get SSID and frequency
	out, err := exec.CommandContext(ctx, "ifconfig", iface).Output()
	if err != nil {
		return nil, err
	}

	ifcfg := string(out)

	// ignore if not associated
	if !strings.Contains(ifcfg, "status: associated") {
		return nil, nil
	}

	var ssid string
	if m := reSSID.FindStringSubmatch(ifcfg); len(m) > 1 {
		ssid = m[1]
	}
	if ssid == "" {
		return nil, nil
	}

	var freq float64
	if m := reChan.FindStringSubmatch(ifcfg); len(m) > 1 {
		mhz, err := strconv.ParseFloat(m[1], 64)
		if err == nil {
			freq = mhz / 1000.0
		}
	}

	signal, bitrate := parseStaInfo(ctx, iface)

	return protocol.WiFiMetric{
		Interface:   iface,
		SSID:        ssid,
		Frequency:   freq,
		SignalLevel: signal,
		LinkQuality: rssiToQuality(signal),
		BitRate:     bitrate,
	}, nil
}

func parseStaInfo(ctx context.Context, iface string) (int, float64) {
	out, err := exec.CommandContext(ctx, "ifconfig", iface, "list", "sta").Output()
	if err != nil {
		return 0, 0
	}
	return parseStaInfoFrom(string(out))
}

// parseStaInfoFrom parses "ifconfig <iface> link sta" output.
func parseStaInfoFrom(output string) (signal int, bitrate float64) {
	scanner := bufio.NewScanner(strings.NewReader(output))
	// Skip header
	if !scanner.Scan() {
		return 0, 0
	}

	if scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		// ADDR, AID, CHAN, RATE, RSSI, IDLE, TXSEQ, RXSEQ, CAPS, FLAG
		if len(fields) < 5 {
			return 0, 0
		}

		rateStr := strings.TrimSuffix(fields[3], "M")
		if v, err := strconv.ParseFloat(rateStr, 64); err == nil {
			bitrate = v
		}
		if v, err := strconv.Atoi(fields[4]); err == nil {
			signal = v
		}
	}

	return signal, bitrate
}

// rssiToQuality converts RSSI (dBm) to a 0-70 quality scale
// similar to Linux /proc/net/wireless.
func rssiToQuality(rssi int) int {
	if rssi == 0 {
		return 0
	}

	// Clamp to -110 (worst) to -30 (best)
	quality := 2 * (rssi + 110)
	switch {
	case quality < 0:
		return 0
	case quality > 70:
		return 70
	default:
		return quality
	}
}
