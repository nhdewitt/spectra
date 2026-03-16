//go:build freebsd

package network

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"unsafe"

	"golang.org/x/net/route"
	"golang.org/x/sys/unix"
)

const (
	rtmIfInfo   = 0x0e
	ifMsgHdrLen = 16
	ifDataSize  = int(unsafe.Sizeof(ifData{}))
	typeOff     = 3
	indexOff    = 12
)

// collectRaw gathers counters for all network interfaces using
// net.Interfaces() (metadata) and sysctl (traffic stats).
func collectRaw() (map[string]Raw, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("listing interfaces: %w", err)
	}

	stats, err := getAllIfStats()
	if err != nil {
		return nil, fmt.Errorf("getting interface stats: %w", err)
	}

	result := make(map[string]Raw, len(ifaces))

	for _, iface := range ifaces {
		// Skip loopback
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		raw := Raw{
			Interface: iface.Name,
			MAC:       strings.ToUpper(iface.HardwareAddr.String()),
			MTU:       uint32(iface.MTU),
		}

		if s, ok := stats[iface.Index]; ok {
			raw.Speed = s.Baudrate
			raw.RxBytes = s.Ibytes
			raw.RxPackets = s.Ipackets
			raw.RxErrors = s.Ierrors
			raw.RxDrops = s.Iqdrops
			raw.TxBytes = s.Obytes
			raw.TxPackets = s.Opackets
			raw.TxErrors = s.Oerrors
			raw.TxDrops = s.Oqdrops
		}

		result[iface.Name] = raw
	}

	return result, nil
}

type ifStats struct {
	Baudrate uint64
	Ipackets uint64
	Ierrors  uint64
	Opackets uint64
	Oerrors  uint64
	Ibytes   uint64
	Obytes   uint64
	Iqdrops  uint64
	Oqdrops  uint64
}

// getAllIfStats retrieves stats for all interfaces via the routing socket.
func getAllIfStats() (map[int]ifStats, error) {
	rib, err := route.FetchRIB(unix.AF_UNSPEC, route.RIBTypeInterface, 0)
	if err != nil {
		return nil, fmt.Errorf("FetchRIB: %w", err)
	}

	result := make(map[int]ifStats)

	for off := 0; off+ifMsgHdrLen <= len(rib); {
		msgLen := int(binary.LittleEndian.Uint16(rib[off : off+2]))
		if msgLen < ifMsgHdrLen || off+msgLen > len(rib) {
			break
		}

		if rib[off+typeOff] == rtmIfInfo && msgLen >= ifMsgHdrLen+ifDataSize {
			index := int(binary.LittleEndian.Uint16(rib[off+indexOff : off+indexOff+2]))

			var d ifData

			r := bytes.NewReader(rib[off+ifMsgHdrLen : off+ifMsgHdrLen+ifDataSize])
			if err := binary.Read(r, binary.LittleEndian, &d); err == nil {
				result[index] = ifStats{
					Baudrate: d.Baudrate,
					Ipackets: d.Ipackets,
					Ierrors:  d.Ierrors,
					Opackets: d.Opackets,
					Oerrors:  d.Oerrors,
					Ibytes:   d.Ibytes,
					Obytes:   d.Obytes,
					Iqdrops:  d.Iqdrops,
					Oqdrops:  d.Oqdrops,
				}
			}
		}

		off += msgLen
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
