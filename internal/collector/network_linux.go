//go:build linux

package collector

import (
	"bufio"
	"io"
	"os"
	"strconv"
	"strings"
)

// collectNetworkRaw reads cumulative counters from /proc/net/dev
// and adds MAC, MTU, and link speed from /sys/class/net.
func collectNetworkRaw() (map[string]NetworkRaw, error) {
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
