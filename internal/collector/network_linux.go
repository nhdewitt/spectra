//go:build linux

package collector

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

// NetworkRaw holds the cumulative counters from /proc/net/dev
type NetworkRaw struct {
	Interface string
	MAC       string
	MTU       uint32
	Speed     uint64
	RxBytes   uint64 // fields[0]
	RxPackets uint64 // fields[1]
	RxErrors  uint64 // fields[2]
	RxDrops   uint64 // fields[3]
	TxBytes   uint64 // fields[8]
	TxPackets uint64 // fields[9]
	TxErrors  uint64 // fields[10]
	TxDrops   uint64 // fields[11]
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

func CollectNetwork(ctx context.Context) ([]protocol.Metric, error) {
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
			RxErrors:  delta(curr.RxErrors, prev.RxErrors),
			RxDrops:   rate(delta(curr.RxDrops, prev.RxDrops), elapsed),
			TxBytes:   rate(delta(curr.TxBytes, prev.TxBytes), elapsed),
			TxPackets: delta(curr.TxPackets, prev.TxPackets),
			TxErrors:  delta(curr.TxErrors, prev.TxErrors),
			TxDrops:   delta(curr.TxDrops, prev.TxDrops),
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

	for scanner.Scan() {
		line := scanner.Text()

		if strings.TrimSpace(line) == "" {
			continue
		}

		split := strings.SplitN(line, ":", 2)
		if len(split) != 2 {
			continue
		}

		iface := strings.TrimSpace(split[0])
		values := strings.Fields(split[1])

		if len(values) < 16 {
			continue
		}

		raw := NetworkRaw{
			Interface: iface,
		}

		parse := makeUintParser(values, "/proc/net/dev:"+iface)

		// /proc/net/dev standard:
		// 0: bytes_in, 1: packets_in, 2: errs_in 3: drops_in
		// 8: bytes_out, 9: packets_out, 10: errs_out 11: drops_out

		raw.RxBytes = parse(0)
		raw.RxPackets = parse(1)
		raw.RxErrors = parse(2)
		raw.RxDrops = parse(3)

		raw.TxBytes = parse(8)
		raw.TxPackets = parse(9)
		raw.TxErrors = parse(10)
		raw.TxDrops = parse(11)

		raw.MAC = strings.ToUpper(getLinuxMAC(iface))
		raw.MTU = getLinuxMTU(iface)
		raw.Speed = getLinuxLinkSpeed(iface)

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

func getLinuxMAC(ifaceName string) string {
	path := "/sys/class/net/" + ifaceName + "/address"
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(data))
}

func getLinuxMTU(ifaceName string) uint32 {
	path := "/sys/class/net/" + ifaceName + "/mtu"
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	val, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 32)
	if err != nil {
		return 0
	}

	return uint32(val)
}

func getLinuxLinkSpeed(ifaceName string) uint64 {
	path := "/sys/class/net/" + ifaceName + "/speed"
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}

	valStr := strings.TrimSpace(string(data))
	speedMbit, err := strconv.ParseUint(valStr, 10, 64)
	if err != nil {
		return 0
	}

	return speedMbit * 1_000_000
}
