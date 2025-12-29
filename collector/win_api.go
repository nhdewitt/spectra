//go:build windows

package collector

import "syscall"

// --- DLL Handles ---
var (
	ntdll    = syscall.NewLazyDLL("ntdll.go")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	wlanapi  = syscall.NewLazyDLL("wlanapi.dll")
)

// --- Procedure Handles ---
var (
	// CPU/System

	procNtQuerySystemInformation = ntdll.NewProc("NtQuerySystemInformation")
	procGetNativeSystemInfo      = kernel32.NewProc("GetNativeSystemInfo")

	// Memory
	procGlobalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")

	// WLAN
	wlanOpenHandle     = wlanapi.NewProc("WlanOpenHandle")
	wlanCloseHandle    = wlanapi.NewProc("WlanCloseHandle")
	wlanEnumInterfaces = wlanapi.NewProc("WlanEnumInterfaces")
	wlanQueryInterface = wlanapi.NewProc("WlanQueryInterface")
	wlanFreeMemory     = wlanapi.NewProc("WlanFreeMemory")
)

// --- Constants ---
const (
	// CPU/System

	systemProcessorPerformanceInformation = 8

	// WLAN

	// WLAN_API_VERSION_2_0 is for Windows Vista+
	wlanApiVersion = 2

	// OpCodes from wlanapi.h WLAN_INTF_OPCODE enum

	wlanIntfOpcodeCurrentConnection = 0x00000007
	wlanIntfOpcodeChannelNumber     = 0x00000008
	wlanIntfOpcodeRssi              = 0x10000102
)

// --- Struct Definitions ---

// CPU: Native internal structure
type systemProcessorPerformanceInfo struct {
	IdleTime       int64
	KernelTime     int64
	UserTime       int64
	DpcTime        int64
	InterruptTime  int64
	InterruptCount uint32
	_              uint32 // Padding for 64-bit alignment
}

// CPU: Hardware topology
type systemInfo struct {
	ProcessorArchitecture     uint16
	Reserved                  uint16
	PageSize                  uint32
	MinimumApplicationAddress uintptr
	MaximumApplicationAddress uintptr
	ActiveProcessorMask       uintptr
	NumberOfProcessors        uint32
	ProcessorType             uint32
	AllocationGranularity     uint32
	ProcessorLevel            uint16
	ProcessorRevision         uint16
}

// Memory: RAM and Pagefile
type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

// WLAN: Array of interface info
type WLAN_INTERFACE_INFO_LIST struct {
	NumberOfItems uint32
	Index         uint32
	InterfaceInfo [1]WLAN_INTERFACE_INFO
}

// WLAN: Interface
type WLAN_INTERFACE_INFO struct {
	InterfaceGuid        GUID
	InterfaceDescription [256]uint16
	IsState              uint32
}

// WLAN: Interface GUID
type GUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

// WLAN: Wireless connection attributes
type WLAN_CONNECTION_ATTRIBUTES struct {
	IsState                   uint32
	wlanConnectionMode        uint32
	strProfileName            [256]uint16
	wlanAssociationAttributes WLAN_ASSOCIATION_ATTRIBUTES
	wlanSecurityAttributes    WLAN_SECURITY_ATTRIBUTES
}

// WLAN: Connection association attributes
type WLAN_ASSOCIATION_ATTRIBUTES struct {
	dot11Ssid         DOT11_SSID
	dot11BssType      uint32
	dot11Bssid        [6]byte
	dot11PhyType      uint32
	uDot11PhyIndex    uint32
	wlanSignalQuality uint32
	ulRxRate          uint32
	ulTxRate          uint32
}

// WLAN: SSID
type DOT11_SSID struct {
	uSSIDLength uint32
	ucSSID      [32]byte
}

// WLAN: Security
type WLAN_SECURITY_ATTRIBUTES struct {
	bSecurityEnabled     uint32
	bOneXEnabled         uint32
	dot11AuthAlgorithm   uint32
	dot11CipherAlgorithm uint32
}
