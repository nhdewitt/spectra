//go:build linux

package wifi

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/nhdewitt/spectra/internal/protocol"
)

// wifiMeta is the association state of one interface, as reported by `iw`.
// The zero value means the interface exists but is not associated to any
// network, distinct from a fetch failure.
type wifiMeta struct {
	SSID    string
	Freq    float64
	BitRate float64
}

// Associated reports whether the interface is joined to a network.
func (m wifiMeta) Associated() bool { return m.SSID != "" }

type metadataFetcher func(ctx context.Context, iface string) (wifiMeta, error)

var (
	reSSID    = regexp.MustCompile(`SSID: (.+)`)
	reFreq    = regexp.MustCompile(`freq: (\d+)`)
	reBitRate = regexp.MustCompile(`tx bitrate: ([\d.]+)`)
)

func Collect(ctx context.Context) ([]protocol.Metric, error) {
	return parseNetWireless(ctx, getWiFiMetadata)
}

func parseNetWireless(ctx context.Context, fetcher metadataFetcher) ([]protocol.Metric, error) {
	f, err := os.Open("/proc/net/wireless")
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No Wi-Fi and kernel built without CONFIG_WIRELESS
		}
		return nil, fmt.Errorf("parsing /proc/net/wireless: %w", err)
	}
	defer f.Close()

	return parseNetWirelessFrom(ctx, f, fetcher)
}

func parseNetWirelessFrom(ctx context.Context, r io.Reader, fetcher metadataFetcher) ([]protocol.Metric, error) {
	var results []protocol.Metric
	var fetchErrs []error
	scanner := bufio.NewScanner(r)

	for range 2 {
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return nil, fmt.Errorf("reading /proc/net/wireless header: %w", err)
			}
			return nil, errors.New("/proc/net/wireless ended before its two header lines")
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

		meta, err := fetcher(ctx, iface)
		if err != nil {
			// iw failed for this interface. Different from an idle interface.
			fetchErrs = append(fetchErrs, fmt.Errorf("interface %s: %w", iface, err))
			continue
		}

		if !meta.Associated() {
			continue
		}

		metric := protocol.WiFiMetric{
			Interface:   iface,
			SignalLevel: int(sigLevel),
			LinkQuality: int(linkQual),
			SSID:        meta.SSID,
			Frequency:   meta.Freq,
			BitRate:     meta.BitRate,
		}

		results = append(results, metric)
	}

	return results, errors.Join(append(fetchErrs, scanner.Err())...)
}

// parseFloat strips trailing dots before parsing.
func parseFloat(s string) (float64, error) {
	return strconv.ParseFloat(strings.TrimSuffix(s, "."), 64)
}

// getWiFiMetadata calls `iwgetid` to fetch SSID and Frequency
func getWiFiMetadata(ctx context.Context, iface string) (wifiMeta, error) {
	// iw dev <interface> link
	out, err := exec.CommandContext(ctx, "iw", "dev", iface, "link").Output()
	if err != nil {
		return wifiMeta{}, fmt.Errorf("running iw dev %s link: %w", iface, err)
	}

	return parseIWLink(string(out)), nil
}

// parseIWLink extracts association state from `iw dev <iface> link` output.
// Absent fields are left at zero rather than treated as errors (iw omits
// bitrate on a freshly associated interface and omits everything when the
// interface is idle).
func parseIWLink(output string) wifiMeta {
	var meta wifiMeta

	// Parse SSID
	if match := reSSID.FindStringSubmatch(output); len(match) > 1 {
		meta.SSID = strings.TrimSpace(match[1])
	}

	// Parse Frequency
	if match := reFreq.FindStringSubmatch(output); len(match) > 1 {
		val, _ := strconv.ParseFloat(match[1], 64)
		meta.Freq = val / 1000.0
	}

	// Parse Bitrate
	if match := reBitRate.FindStringSubmatch(output); len(match) > 1 {
		if val, err := strconv.ParseFloat(match[1], 64); err == nil {
			meta.BitRate = val
		}
	}

	return meta
}
