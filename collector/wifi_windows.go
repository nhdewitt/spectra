//go:build windows

package collector

import (
	"context"
	"fmt"
	"unsafe"

	"github.com/nhdewitt/spectra/metrics"
	"golang.org/x/sys/windows"
)

func CollectWiFi(ctx context.Context) ([]metrics.Metric, error) {
	var handle uintptr
	var negotiatedVersion uint32

	ret, _, _ := wlanOpenHandle.Call(
		wlanApiVersion, // [in] DWORD dwClientVersion (2)
		0,              // [in] PVOID (0)
		uintptr(unsafe.Pointer(&negotiatedVersion)), // [out] PDWORD (result)
		uintptr(unsafe.Pointer(&handle)),            // [out] PHANDLE (handle)
	)
	if ret != 0 {
		return nil, fmt.Errorf("WlanOpenHandle failed: %d", ret)
	}
	defer wlanCloseHandle.Call(handle, 0)

	var ifaceList *WLAN_INTERFACE_INFO_LIST
	ret, _, _ = wlanEnumInterfaces.Call(
		handle,                              // [in] HANDLE
		0,                                   // [in] PVOID
		uintptr(unsafe.Pointer(&ifaceList)), // [out] PWLAN_INTERFACE_INFO_LIST
	)
	if ret != 0 {
		return nil, fmt.Errorf("WlanEnumInterfaces failed: %d", ret)
	}

	// Verify pointer
	if ifaceList == nil {
		return nil, nil
	}
	defer wlanFreeMemory.Call(uintptr(unsafe.Pointer(ifaceList)))

	start := &ifaceList.InterfaceInfo[0]
	ifaces := unsafe.Slice(start, ifaceList.NumberOfItems)

	var results []metrics.Metric

	for i := range ifaces {
		info := &ifaces[i]

		// Check if connected (wlan_interface_state_connected)
		if info.IsState != 1 {
			continue
		}

		name := utf16ToString(info.InterfaceDescription[:])

		// Connection attributes
		var dataSize uint32
		var connAttr *WLAN_CONNECTION_ATTRIBUTES
		var opcode uint32 = wlanIntfOpcodeCurrentConnection

		ret, _, _ := wlanQueryInterface.Call(
			handle, // [in] HANDLE
			uintptr(unsafe.Pointer(&info.InterfaceGuid)), // [in] GUID
			uintptr(opcode),                    // [in] WLAN_INTF_OPCODE
			0,                                  // [in, out] PVOID
			uintptr(unsafe.Pointer(&dataSize)), // [out] PDWORD
			uintptr(unsafe.Pointer(&connAttr)), // [out] PVOID
			0,                                  // [out, opt] PWLAN_OPCODE_VALUE_TYPE
		)

		if ret != 0 {
			continue
		}

		ssid := parseDot11SSID(connAttr.wlanAssociationAttributes.dot11Ssid)
		quality := int(connAttr.wlanAssociationAttributes.wlanSignalQuality)
		txRateKbps := connAttr.wlanAssociationAttributes.ulTxRate
		bitRate := float64(txRateKbps) / 1000.0

		wlanFreeMemory.Call(uintptr(unsafe.Pointer(connAttr)))

		var rssiPtr *int32
		var rssi int
		opcode = wlanIntfOpcodeRssi

		ret, _, _ = wlanQueryInterface.Call(
			handle,
			uintptr(unsafe.Pointer(&info.InterfaceGuid)),
			uintptr(opcode),
			0,
			uintptr(unsafe.Pointer(&dataSize)),
			uintptr(unsafe.Pointer(&rssiPtr)),
			0,
		)

		if ret == 0 && rssiPtr != nil {
			rssi = int(*rssiPtr)
			wlanFreeMemory.Call(uintptr(unsafe.Pointer(rssiPtr)))
		} else {
			// Fallback if wlanIntfOpcodeRssi isn't supported
			// convert & quality to rough dBm
			rssi = (quality / 2) - 100
		}

		var freqPtr *uint32
		var frequency float64

		ret, _, _ = wlanQueryInterface.Call(
			handle,
			uintptr(unsafe.Pointer(&info.InterfaceGuid)),
			uintptr(wlanIntfOpcodeChannelFrequency),
			0,
			uintptr(unsafe.Pointer(&dataSize)),
			uintptr(unsafe.Pointer(&freqPtr)),
			0,
		)

		if ret == 0 && freqPtr != nil {
			freqKHz := *freqPtr
			frequency = float64(freqKHz) / 1_000_000.0
			wlanFreeMemory.Call(uintptr(unsafe.Pointer(freqPtr)))
		} else {
			var channelPtr *uint32
			ret, _, _ = wlanQueryInterface.Call(
				handle,
				uintptr(unsafe.Pointer(&info.InterfaceGuid)),
				uintptr(wlanIntfOpcodeChannelNumber),
				0,
				uintptr(unsafe.Pointer(&dataSize)),
				uintptr(unsafe.Pointer(&channelPtr)),
				0,
			)

			if ret == 0 && channelPtr != nil {
				channel := *channelPtr
				wlanFreeMemory.Call(uintptr(unsafe.Pointer(channelPtr)))

				frequency = channelToFrequency(channel)
			}
		}

		results = append(results, metrics.WiFiMetric{
			Interface:   name,
			SSID:        ssid,
			SignalLevel: rssi,
			LinkQuality: quality,
			Frequency:   frequency,
			BitRate:     bitRate,
		})
	}

	return results, nil
}

func utf16ToString(w []uint16) string {
	return windows.UTF16ToString(w)
}

func parseDot11SSID(ssid DOT11_SSID) string {
	if ssid.uSSIDLength == 0 {
		return ""
	}
	return string(ssid.ucSSID[:ssid.uSSIDLength])
}

// channelToFrequency converts a channel number to its center frequency in GHz.
// Based on IEEE 802.11 formulas.
func channelToFrequency(channel uint32) float64 {
	var mhz int
	switch {
	// 2.4GHz: 2407 + (5 * Channel) MHz
	case channel >= 1 && channel <= 13:
		mhz = 2407 + (5 * int(channel))
	// Japan
	case channel == 14:
		return 2.484
	// 5GHz: 5000 + (5 * Channel) MHz
	case channel >= 32 && channel <= 177:
		mhz = 5000 + (5 * int(channel))
	// 6GHz (avoiding overlapping numbers w/ 2.4): 5950 + (5 * Channel) MHz
	case channel > 177:
		mhz = 5950 + (5 * int(channel))
	default:
		return 0.0
	}
	return float64(mhz) / 1000.0
}
