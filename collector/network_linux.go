//go:build !windows

package collector

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/nhdewitt/raspimon/metrics"
)

// NetworkRaw holds the cumulative counters from /proc/net/dev
type NetworkRaw struct {
	Interface   string // fields[0]
	BytesRcvd   uint64 // fields[1]
	PacketsRcvd uint64 // fields[2]
	ErrorsRcvd  uint64 // fields[3]
	BytesSent   uint64 // fields[9]
	PacketsSent uint64 // fields[10]
	ErrorsSent  uint64 // fields[11]
}

var (
	lastNetworkRaw  map[string]NetworkRaw
	lastNetworkTime time.Time
)

var ignoredInterfacePrefixes = []string{
	"lo",
	"docker",
	"br-",
	"veth",
	"virbr",
	"vmnet",
	"vboxnet",
	"tun",
	"tap",
	"wg",
	"tailscale",
	"nordlynx",
	"flannel",
	"cni",
	"calico",
	"cali",
	"dummy",
	"bond",
}

func CollectNetwork(ctx context.Context) ([]metrics.Metric, error) {
	current, err := parseNetDev()
	if err != nil {
		return nil, fmt.Errorf("parsing /proc/net/dev: %w", err)
	}

	now := time.Now()

	// Baseline
	if len(lastNetworkRaw) == 0 {
		lastNetworkRaw = current
		lastNetworkTime = now
		return nil, nil
	}

	elapsed := now.Sub(lastNetworkTime).Seconds()
	if elapsed <= 0 {
		lastNetworkRaw = nil
		return nil, nil
	}

	var results []metrics.Metric

	for iface, curr := range current {
		prev, ok := lastNetworkRaw[iface]
		if !ok {
			continue
		}

		if shouldIgnoreInterface(iface) {
			continue
		}

		metric := metrics.NetworkMetric{
			Interface:   iface,
			BytesRcvd:   rate(curr.BytesRcvd-prev.BytesRcvd, elapsed),
			BytesSent:   rate(curr.BytesSent-prev.BytesSent, elapsed),
			PacketsRcvd: rate(curr.PacketsRcvd-prev.PacketsRcvd, elapsed),
			PacketsSent: rate(curr.PacketsSent-prev.PacketsSent, elapsed),
			ErrorsRcvd:  curr.ErrorsRcvd - prev.ErrorsRcvd,
			ErrorsSent:  curr.ErrorsSent - prev.ErrorsSent,
		}

		results = append(results, metric)
	}

	lastNetworkRaw = current
	lastNetworkTime = now

	return results, nil
}

func parseNetDev() (map[string]NetworkRaw, error) {
	f, err := os.Open("/proc/net/dev")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return parseNetDevFrom(f)
}

func parseNetDevFrom(r io.Reader) (map[string]NetworkRaw, error) {
	result := make(map[string]NetworkRaw)
	scanner := bufio.NewScanner(r)

	// Skip headers
	for range 2 {
		if !scanner.Scan() {
			return result, scanner.Err()
		}
	}

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)

		if len(fields) < 17 {
			continue
		}

		iface := strings.TrimSuffix(fields[0], ":")
		values := fields
		if strings.HasSuffix(fields[0], ":") {
			values = fields[1:]
		} else if strings.Contains(fields[0], ":") {
			parts := strings.SplitN(fields[0], ":", 2)
			iface = parts[0]
			values = append([]string{parts[1]}, fields[1:]...)
		}

		raw := NetworkRaw{
			Interface: iface,
		}

		parse := makeUintParser(values, "/proc/net/dev:"+iface)

		// /proc/net/dev standard:
		// 0: bytes_in, 1: packets_in, 2: errs_in
		// 8: bytes_out, 9: packets_out, 10: errs_out

		if len(values) < 16 {
			continue
		}

		raw.BytesRcvd = parse(0)
		raw.PacketsRcvd = parse(1)
		raw.ErrorsRcvd = parse(2)

		raw.BytesSent = parse(8)
		raw.PacketsSent = parse(9)
		raw.ErrorsSent = parse(10)

		result[iface] = raw
	}

	return result, scanner.Err()
}

func shouldIgnoreInterface(name string) bool {
	for _, prefix := range ignoredInterfacePrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}
