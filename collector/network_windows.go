//go:build windows

package collector

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/nhdewitt/spectra/metrics"
	"github.com/yusufpapurcu/wmi"
)

var (
	lastWMIStats   map[string]Win32_PerfRawData_Tcpip_NetworkInterface
	lastWMITime    time.Time
	wmiErrorLogged bool
)

type Win32_NetworkAdapter struct {
	Name            string
	NetEnabled      bool
	PhysicalAdapter bool
}

type Win32_PerfRawData_Tcpip_NetworkInterface struct {
	Name                  string
	BytesReceivedPerSec   uint64
	BytesSentPerSec       uint64
	PacketsReceivedPerSec uint64
	PacketsSentPerSec     uint64
	PacketsReceivedErrors uint64
	PacketsOutboundErrors uint64
}

func CollectNetwork(ctx context.Context) ([]metrics.Metric, error) {
	var configDst []Win32_NetworkAdapter

	filterQuery := `SELECT Name, PhysicalAdapter, NetEnabled
					FROM Win32_NetworkAdapter
					WHERE PhysicalAdapter = TRUE AND NetEnabled = TRUE`

	err := wmi.Query(filterQuery, &configDst)
	if err != nil {
		if !wmiErrorLogged {
			log.Printf("WARNING: WMI Network Filter query failed: %v", err)
			wmiErrorLogged = true
		}
		return nil, nil
	}

	allowedNames := make(map[string]bool)
	for _, adapter := range configDst {
		allowedNames[adapter.Name] = true

		fixedName := strings.ReplaceAll(adapter.Name, "(", "[")
		fixedName = strings.ReplaceAll(fixedName, ")", "]")
		allowedNames[fixedName] = true
	}

	var statsDst []Win32_PerfRawData_Tcpip_NetworkInterface

	statsQuery := `SELECT Name, BytesReceivedPersec, BytesSentPersec,
				   PacketsReceivedPersec, PacketsSentPerSec,
				   PacketsReceivedErrors, PacketsOutboundErrors
				   FROM Win32_PerfRawData_Tcpip_NetworkInterface`

	err = wmi.Query(statsQuery, &statsDst)
	if err != nil {
		if !wmiErrorLogged {
			log.Printf("WARNING: WMI Network Stats query failed: %v", err)
			wmiErrorLogged = true
		}
		return nil, nil
	}

	if wmiErrorLogged {
		wmiErrorLogged = false
	}

	now := time.Now()

	currentStats := make(map[string]Win32_PerfRawData_Tcpip_NetworkInterface)
	for _, s := range statsDst {
		currentStats[s.Name] = s
	}

	if len(lastWMIStats) == 0 {
		lastWMIStats = currentStats
		lastWMITime = now
		return nil, nil
	}

	elapsed := now.Sub(lastWMITime).Seconds()
	if elapsed <= 0 {
		lastWMIStats = nil
		return nil, nil
	}

	var results []metrics.Metric

	for name, curr := range currentStats {
		if !allowedNames[name] {
			continue
		}

		prev, ok := lastWMIStats[name]
		if !ok {
			continue
		}

		metric := metrics.NetworkMetric{
			Interface:   name,
			BytesRcvd:   rate(curr.BytesReceivedPerSec-prev.BytesReceivedPerSec, elapsed),
			BytesSent:   rate(curr.PacketsReceivedPerSec-prev.PacketsReceivedPerSec, elapsed),
			PacketsRcvd: rate(curr.PacketsReceivedPerSec-prev.PacketsReceivedPerSec, elapsed),
			PacketsSent: rate(curr.PacketsSentPerSec-prev.PacketsSentPerSec, elapsed),
			ErrorsRcvd:  curr.PacketsReceivedErrors - prev.PacketsReceivedErrors,
			ErrorsSent:  curr.PacketsOutboundErrors - prev.PacketsOutboundErrors,
		}

		results = append(results, metric)
	}

	lastWMIStats = currentStats
	lastWMITime = now

	return results, nil
}
