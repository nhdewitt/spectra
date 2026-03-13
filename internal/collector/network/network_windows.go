//go:build windows

package network

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"
	"unsafe"

	"github.com/nhdewitt/spectra/internal/protocol"
	"github.com/nhdewitt/spectra/internal/util"
	"github.com/nhdewitt/spectra/internal/winapi"
	"golang.org/x/sys/windows"
)

var (
	lastNetStats map[uint32]interfaceState
	lastNetTime  time.Time
	nowFunc      = time.Now
)

var ignoredNetworkKeywords = []string{
	"Filter", "Npcap", "QoS", "Virtual Switch", "Pseudo-Interface",
	"Miniport", "Kernel Debug", "Teredo", "IP-HTTPS", "6to4", "Virtual Ethernet",
}

type interfaceState struct {
	raw  winapi.MibIfRow2
	name string
}

func Collect(ctx context.Context) ([]protocol.Metric, error) {
	var tablePtr *winapi.MibIfTable2
	ret, _, _ := winapi.ProcGetIfTable2.Call(uintptr(unsafe.Pointer(&tablePtr)))
	if ret != 0 {
		return nil, fmt.Errorf("GetIfTable2 failed with error code %d", ret)
	}
	defer winapi.ProcFreeMibTable.Call(uintptr(unsafe.Pointer(tablePtr)))

	now := nowFunc()
	currentStats := make(map[uint32]interfaceState)

	start := &tablePtr.Table[0]
	rows := unsafe.Slice(start, tablePtr.NumEntries)

	for i := range rows {
		rowPtr := &rows[i]

		// Filter up interfaces (IfOperStatusUp = 1) and ignore lo (IfTypeSoftwareLoopback = 24)
		if rowPtr.OperStatus != 1 || rowPtr.Type == 24 {
			continue
		}

		// Ignore empty data (e.g. virtual adapters)
		if rowPtr.InOctets == 0 && rowPtr.OutOctets == 0 {
			continue
		}

		name := windows.UTF16ToString(rowPtr.Description[:])
		if name == "" {
			name = windows.UTF16ToString(rowPtr.Alias[:])
		}

		if isIgnoredInterface(name) {
			continue
		}

		currentStats[rowPtr.InterfaceIndex] = interfaceState{
			raw:  *rowPtr,
			name: name,
		}
	}

	// Baseline
	if lastNetStats == nil {
		lastNetStats = currentStats
		lastNetTime = now
		return nil, nil
	}

	// Time Delta Calculation
	secondsElapsed := util.ValidateTimeDelta(now, lastNetTime, "network")
	if secondsElapsed == 0 {
		lastNetStats = currentStats
		lastNetTime = now
		return nil, nil
	}

	var result []protocol.Metric

	for idx, curr := range currentStats {
		prev, ok := lastNetStats[idx]
		if !ok {
			continue
		}

		errsIn := curr.raw.InErrors - prev.raw.InErrors
		errsOut := curr.raw.OutErrors - prev.raw.OutErrors
		dropIn := curr.raw.InDiscards - prev.raw.InDiscards
		dropOut := curr.raw.OutDiscards - prev.raw.OutDiscards

		speed := curr.raw.ReceiveLinkSpeed
		// Guard against -1 overflow
		if speed == ^uint64(0) {
			speed = 0
		}

		result = append(result, protocol.NetworkMetric{
			Interface: curr.name,
			MAC:       strings.ToUpper(formatMAC(curr.raw.PhysicalAddress, curr.raw.PhysicalAddressLength)),
			MTU:       curr.raw.Mtu,
			Speed:     speed,
			RxBytes:   util.Rate(curr.raw.InOctets-prev.raw.InOctets, secondsElapsed),
			RxPackets: util.Rate(curr.raw.InUcastPkts-prev.raw.InUcastPkts, secondsElapsed),
			RxErrors:  util.Rate(errsIn, secondsElapsed),
			RxDrops:   util.Rate(dropIn, secondsElapsed),
			TxBytes:   util.Rate(curr.raw.OutOctets-prev.raw.OutOctets, secondsElapsed),
			TxPackets: util.Rate(curr.raw.OutUcastPkts-prev.raw.OutUcastPkts, secondsElapsed),
			TxErrors:  util.Rate(errsOut, secondsElapsed),
			TxDrops:   util.Rate(dropOut, secondsElapsed),
		})
	}

	lastNetStats = currentStats
	lastNetTime = now
	return result, nil
}

func isIgnoredInterface(name string) bool {
	if strings.HasSuffix(name, "-0000") {
		return true
	}

	for _, keyword := range ignoredNetworkKeywords {
		if strings.Contains(name, keyword) {
			return true
		}
	}

	return false
}

func formatMAC(macArr [32]byte, length uint32) string {
	if length == 0 || length > 32 {
		return ""
	}
	return net.HardwareAddr(macArr[:length]).String()
}
