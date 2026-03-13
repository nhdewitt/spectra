//go:build linux || freebsd

package network

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
	"github.com/nhdewitt/spectra/internal/util"
)

// Raw holds the cumulative counters for a single interface.
type Raw struct {
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
	lastRaw         map[string]Raw
	lastNetworkTime time.Time
)

var ignoredInterfacePrefixes = []string{
	"lo", "docker", "br-", "veth", "virbr", "vmnet", "vboxnet", "tun", "tap", "wg",
	"tailscale", "nordlynx", "flannel", "cni", "calico", "cali", "dummy", "bond",
}

func Collect(ctx context.Context) ([]protocol.Metric, error) {
	current, err := collectRaw()
	if err != nil {
		return nil, fmt.Errorf("collecting network stats: %w", err)
	}

	now := time.Now()

	// Baseline
	if len(lastRaw) == 0 {
		lastRaw = current
		lastNetworkTime = now
		return nil, nil
	}

	elapsed := now.Sub(lastNetworkTime).Seconds()
	if elapsed <= 0 {
		lastRaw = nil
		return nil, nil
	}

	var results []protocol.Metric

	for iface, curr := range current {
		prev, ok := lastRaw[iface]
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
			RxBytes:   util.Rate(util.Delta(curr.RxBytes, prev.RxBytes), elapsed),
			RxPackets: util.Rate(util.Delta(curr.RxPackets, prev.RxPackets), elapsed),
			RxErrors:  util.Rate(util.Delta(curr.RxErrors, prev.RxErrors), elapsed),
			RxDrops:   util.Rate(util.Delta(curr.RxDrops, prev.RxDrops), elapsed),
			TxBytes:   util.Rate(util.Delta(curr.TxBytes, prev.TxBytes), elapsed),
			TxPackets: util.Rate(util.Delta(curr.TxPackets, prev.TxPackets), elapsed),
			TxErrors:  util.Rate(util.Delta(curr.TxErrors, prev.TxErrors), elapsed),
			TxDrops:   util.Rate(util.Delta(curr.TxDrops, prev.TxDrops), elapsed),
		}

		results = append(results, metric)
	}

	lastRaw = current
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
