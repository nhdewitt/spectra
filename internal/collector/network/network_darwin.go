//go:build darwin

package network

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"syscall"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
	"github.com/nhdewitt/spectra/internal/util"
	"golang.org/x/net/route"
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

// ifMsghdr2 mirrors the if_msghdr2 struct from net/if.h.
type ifMsghdr2 struct {
	Msglen    uint16
	Version   uint8
	Type      uint8
	Addrs     int32
	Flags     int32
	Index     uint16
	_         [2]byte // Padding
	SndLen    int32
	SndMaxLen int32
	SndDrops  int32
	Timer     int32
	Data      ifData64
}

// ifData64 mirrors the if_data64 struct from net/if_var.h.
type ifData64 struct {
	Type       uint8
	Typelen    uint8
	Physical   uint8
	Addrlen    uint8
	Hdrlen     uint8
	Recvquota  uint8
	Xmitquota  uint8
	Unused1    uint8
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
	Noproto    uint64
	Recvtiming uint32
	Xmittiming uint32
	_          [8]byte // Padding
}

type ifInfo struct {
	Name string
	MAC  string
	MTU  uint32
}

var ignoredInterfacePrefixes = []string{
	"lo", "docker", "br-", "veth", "virbr", "vmnet", "vboxnet",
	"tun", "tap", "wg", "tailscale", "nordlynx", "flannel",
	"cni", "calico", "cali", "dummy", "bond", "bridge", "gif",
	"stf", "utun", "awdl", "llw", "ap", "anpi", "XHC",
}

func Collect(ctx context.Context) ([]protocol.Metric, error) {
	current, err := collectNetworkRaw()
	if err != nil {
		return nil, fmt.Errorf("collecting network stats: %w", err)
	}

	now := time.Now()

	if len(lastNetworkRaw) == 0 {
		lastNetworkRaw = current
		lastNetworkTime = now
		return nil, nil
	}

	elapsed := now.Sub(lastNetworkTime).Seconds()
	if elapsed <= 0 {
		return nil, nil
	}

	var results []protocol.Metric
	for iface, cur := range current {
		prev, ok := lastNetworkRaw[iface]
		if !ok {
			continue
		}

		metric := protocol.NetworkMetric{
			Interface: iface,
			MAC:       cur.MAC,
			MTU:       cur.MTU,
			Speed:     cur.Speed,
			RxBytes:   util.Rate(util.Delta(cur.RxBytes, prev.RxBytes), elapsed),
			RxPackets: util.Rate(util.Delta(cur.RxPackets, prev.RxPackets), elapsed),
			RxErrors:  util.Rate(util.Delta(cur.RxErrors, prev.RxErrors), elapsed),
			RxDrops:   util.Rate(util.Delta(cur.RxDrops, prev.RxDrops), elapsed),
			TxBytes:   util.Rate(util.Delta(cur.TxBytes, prev.TxBytes), elapsed),
			TxPackets: util.Rate(util.Delta(cur.TxPackets, prev.TxPackets), elapsed),
			TxErrors:  util.Rate(util.Delta(cur.TxErrors, prev.TxErrors), elapsed),
			TxDrops:   util.Rate(util.Delta(cur.TxDrops, prev.TxDrops), elapsed),
		}

		results = append(results, metric)
	}

	lastNetworkRaw = current
	lastNetworkTime = now

	return results, nil
}

// collectNetworkRaw fetches network counters via NET_RT_IFLIST2.
func collectNetworkRaw() (map[string]NetworkRaw, error) {
	// Build index->ifInfo from net.Interfaces()
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("net.interfaces: %w", err)
	}

	infoIdx := make(map[uint16]ifInfo, len(ifaces))
	for _, iface := range ifaces {
		if shouldIgnoreInterface(iface.Name) {
			continue
		}

		infoIdx[uint16(iface.Index)] = ifInfo{
			Name: iface.Name,
			MAC:  strings.ToUpper(iface.HardwareAddr.String()),
			MTU:  uint32(iface.MTU),
		}
	}

	// fetch RIB with NET_RF_IFLIST2
	rib, err := route.FetchRIB(syscall.AF_UNSPEC, syscall.NET_RT_IFLIST2, 0)
	if err != nil {
		return nil, fmt.Errorf("FetchRIB NET_RT_IFLIST2: %w", err)
	}

	result := make(map[string]NetworkRaw)

	for len(rib) > 0 {
		if len(rib) < 4 {
			break
		}

		msgLen := binary.LittleEndian.Uint16(rib[:2])
		if int(msgLen) > len(rib) || msgLen == 0 {
			break
		}

		msgType := rib[3]

		// 0x12 carries if_msghdr2
		structSize := binary.Size(ifMsghdr2{})
		if msgType == syscall.RTM_IFINFO2 && int(msgLen) >= structSize {
			var msg ifMsghdr2
			if err := binary.Read(bytes.NewReader(rib[:structSize]), binary.LittleEndian, &msg); err == nil {
				info, ok := infoIdx[msg.Index]
				if ok && (msg.Data.Ibytes > 0 || msg.Data.Obytes > 0) {
					// If no data has been sent, assume we can ignore the interface
					result[info.Name] = NetworkRaw{
						Interface: info.Name,
						MAC:       info.MAC,
						MTU:       info.MTU,
						Speed:     msg.Data.Baudrate,
						RxBytes:   msg.Data.Ibytes,
						RxPackets: msg.Data.Ipackets,
						RxErrors:  msg.Data.Ierrors,
						RxDrops:   msg.Data.Iqdrops,
						TxBytes:   msg.Data.Obytes,
						TxPackets: msg.Data.Opackets,
						TxErrors:  msg.Data.Oerrors,
						TxDrops:   0, // Darwin doesn't track tx drops separately
					}
				}
			}
		}

		rib = rib[msgLen:]
	}

	return result, nil
}

func shouldIgnoreInterface(iface string) bool {
	for _, prefix := range ignoredInterfacePrefixes {
		if strings.HasPrefix(iface, prefix) {
			return true
		}
	}
	return false
}
