//go:build freebsd

package collector

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"strings"

	"golang.org/x/sys/unix"
)

// collectNetworkRaw gathers counters for all network interfaces using
// net.Interfaces() (metadata) and sysctl (traffic stats).
func collectNetworkRaw() (map[string]NetworkRaw, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("listing interfaces: %w", err)
	}

	result := make(map[string]NetworkRaw, len(ifaces))

	for _, iface := range ifaces {
		// Skip loopback
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		raw := NetworkRaw{
			Interface: iface.Name,
			MAC:       strings.ToUpper(iface.HardwareAddr.String()),
			MTU:       uint32(iface.MTU),
		}

		data, err := getIfData(iface.Index)
		if err != nil {
			continue
		}

		raw.Speed = data.Baudrate
		raw.RxBytes = data.Ibytes
		raw.RxPackets = data.Ipackets
		raw.RxErrors = data.Ierrors
		raw.RxDrops = data.Iqdrops
		raw.TxBytes = data.Obytes
		raw.TxPackets = data.Opackets
		raw.TxErrors = data.Oerrors
		raw.TxDrops = data.Oqdrops

		result[iface.Name] = raw
	}

	return result, nil
}

// ifData mirrors the FreeBSD kernel struct if_data
// Source: sys/net/if.h
type ifData struct {
	Type       uint8
	Physical   uint8
	Addrlen    uint8
	Hdrlen     uint8
	LinkState  uint8
	Vhid       uint8
	Datalen    uint16
	Mtu        uint32
	Metric     uint32
	Baudrate   uint64
	Ipackets   uint64
	Ierrors    uint64
	Opackets   uint64
	Oerrors    uint64
	Collisions uint64
	Ibytes     uint64
	Obytes     uint64
	Imcasts    uint64
	Omcasts    uint64
	Iqdrops    uint64
	Oqdrops    uint64
	Noproto    uint64
}

// getIfData returns interface statistics via sysctl for the given interface index.
func getIfData(index int) (*ifData, error) {
	name := fmt.Sprintf("net.link.generic.system.ifdata.%d", index)
	data, err := unix.SysctlRaw(name)
	if err != nil {
		return nil, fmt.Errorf("sysctl: %s: %w", name, err)
	}

	var d ifData
	if err := binary.Read(bytes.NewReader(data), binary.LittleEndian, &d); err != nil {
		return nil, fmt.Errorf("sysctl %s: parsing if_data: %w", name, err)
	}

	return &d, nil
}
