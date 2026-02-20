//go:build linux || freebsd

package collector

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

// NetworkRaw holds the cumulative counters for a single interface.
type NetworkRaw struct {
	Interface string
	MAC       string
	MTU       uint32
	Speed     uint64
	RxBytes   uint64
	RxPackets uint64
	RxErrors  uint64
	RxDrops   uint64
	TxBytes   uint64
	TxPackets uint64
	TxErrors  uint64
	TxDrops   uint64
}

var (
	lastNetworkRaw  map[string]NetworkRaw
	lastNetworkTime time.Time
)

var ignoredInterfacePrefixes = []string{
	"lo", "docker", "br-", "veth", "virbr", "vmnet", "vboxnet", "tun", "tap", "wg",
	"tailscale", "nordlynx", "flannel", "cni", "calico", "cali", "dummy", "bond",
}

func CollectNetwork(ctx context.Context) ([]protocol.Metric, error) {
	current, err := collectNetworkRaw()
	if err != nil {
		return nil, fmt.Errorf("collecting network stats: %w", err)
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

	var results []protocol.Metric

	for iface, curr := range current {
		prev, ok := lastNetworkRaw[iface]
		if !ok {
			continue
		}
		if shouldIgnoreInterface(iface) {
			continue
		}

		metric := protocol.NetworkMetric{
			Interface: iface,
			MAC:       curr.MAC,
			MTU:       curr.MTU,
			Speed:     curr.Speed,
			RxBytes:   rate(delta(curr.RxBytes, prev.RxBytes), elapsed),
			RxPackets: rate(delta(curr.RxPackets, prev.RxPackets), elapsed),
			RxErrors:  rate(delta(curr.RxErrors, prev.RxErrors), elapsed),
			RxDrops:   rate(delta(curr.RxDrops, prev.RxDrops), elapsed),
			TxBytes:   rate(delta(curr.TxBytes, prev.TxBytes), elapsed),
			TxPackets: rate(delta(curr.TxPackets, prev.TxPackets), elapsed),
			TxErrors:  rate(delta(curr.TxErrors, prev.TxErrors), elapsed),
			TxDrops:   rate(delta(curr.TxDrops, prev.TxDrops), elapsed),
		}

		results = append(results, metric)
	}

	lastNetworkRaw = current
	lastNetworkTime = now

	return results, nil
}

func shouldIgnoreInterface(iface string) bool {
	for _, prefix := range ignoredInterfacePrefixes {
		if strings.HasPrefix(iface, prefix) {
			return true
		}
	}
	return false
}
