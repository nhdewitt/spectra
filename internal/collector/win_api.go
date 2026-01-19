//go:build windows

package collector

import (
	"golang.org/x/sys/windows"
)

// BusType represents the hardware interface (USB, SATA, NVMe, etc.)
type BusType uint32

const (
	// CPU/System

	systemProcessorPerformanceInformation = 8

	// Disk: Drive Types

	driveUnknown   = 0
	driveNoRootDir = 1
	driveRemovable = 2
	driveFixed     = 3
	driveRemote    = 4
	driveCdrom     = 5
	driveRamdisk   = 6

	// Disk: IOCTL Codes

	// DeviceType: 7 (Disk), Function: 8, Method: 0 (Buffered), Access: 0 (Any)
	ioctlDiskPerformance            = 0x70020
	ioctlStorageQueryProperty       = 0x2D1400
	ioctlVolumeGetVolumeDiskExtents = 0x560000

	// Disk: Property Types

	storageDeviceProperty = 0 // PropertyId
	storageStandardQuery  = 0 // QueryType

	// Disk: Bus Types

	BusTypeUnknown           BusType = 0x00
	BusTypeScsi              BusType = 0x01
	BusTypeAtapi             BusType = 0x02
	BusTypeAta               BusType = 0x03
	BusType1394              BusType = 0x04
	BusTypeSsa               BusType = 0x05
	BusTypeFibre             BusType = 0x06
	BusTypeUsb               BusType = 0x07
	BusTypeRAID              BusType = 0x08
	BusTypeiScsi             BusType = 0x09
	BusTypeSas               BusType = 0x0A
	BusTypeSata              BusType = 0x0B
	BusTypeSd                BusType = 0x0C
	BusTypeMmc               BusType = 0x0D
	BusTypeVirtual           BusType = 0x0E
	BusTypeFileBackedVirtual BusType = 0x0F
	BusTypeNvme              BusType = 0x11

	// WLAN

	wlanApiVersion = 2 // Windows Vista+

	// WLAN_INTF_OPCODE enum

	wlanIntfOpcodeCurrentConnection = 0x00000007
	wlanIntfOpcodeChannelNumber     = 0x00000008
	wlanIntfOpcodeChannelFrequency  = 0x00000067 // Windows 10+
	wlanIntfOpcodeRssi              = 0x10000102
)

// --- Struct Definitions ---

// CPU

// Native internal structure for NtQuerySystemInformation
type systemProcessorPerformanceInfo struct {
	IdleTime       int64
	KernelTime     int64
	UserTime       int64
	DpcTime        int64
	InterruptTime  int64
	InterruptCount uint32
	_              uint32 // Padding for 64-bit alignment
}

// Hardware topology
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

// Disk

type storagePropertyQuery struct {
	PropertyId           uint32
	QueryType            uint32
	AdditionalParameters [1]byte
}

type storageDeviceDescriptor struct {
	Version               uint32
	Size                  uint32
	DeviceType            byte
	DeviceTypeModifier    byte
	RemovableMedia        bool
	CommandQueueing       bool
	VendorIdOffset        uint32
	ProductIdOffset       uint32
	ProductRevisionOffset uint32
	SerialNumberOffset    uint32
	BusType               uint32
	RawPropertiesLength   uint32
}

type diskExtent struct {
	DiskNumber     uint32
	StartingOffset int64
	ExtentLength   int64
}

type volumeDiskExtents struct {
	NumberOfDiskExtents uint32
	Extents             [1]diskExtent
}

type diskPerformance struct {
	BytesRead           int64
	BytesWritten        int64
	ReadTime            int64
	WriteTime           int64
	IdleTime            int64
	ReadCount           uint32
	WriteCount          uint32
	QueueDepth          uint32
	SplitCount          uint32
	QueryTime           int64
	StorageDeviceNumber uint32
	StorageManagerName  [8]uint16
}

// Memory

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

// Network

type mibIfRow2 struct {
	InterfaceLuid               uint64
	InterfaceIndex              uint32
	InterfaceGuid               windows.GUID
	Alias                       [257]uint16
	Description                 [257]uint16
	PhysicalAddressLength       uint32
	PhysicalAddress             [32]byte
	PermanentPhysicalAddress    [32]byte
	Mtu                         uint32
	Type                        uint32
	TunnelType                  uint32
	MediaType                   uint32
	PhysicalMediumType          uint32
	AccessType                  uint32
	DirectionType               uint32
	InterfaceAndOperStatusFlags uint32
	OperStatus                  uint32
	AdminStatus                 uint32
	MediaConnectState           uint32
	NetworkGuid                 windows.GUID
	ConnectionType              uint32
	_                           [4]byte // Padding for 8-byte alignment
	TransmitLinkSpeed           uint64
	ReceiveLinkSpeed            uint64
	InOctets                    uint64
	InUcastPkts                 uint64
	InNUcastPkts                uint64
	InDiscards                  uint64
	InErrors                    uint64
	InUnknownProtos             uint64
	InUcastOctets               uint64
	InMulticastOctets           uint64
	InBroadcastOctets           uint64
	OutOctets                   uint64
	OutUcastPkts                uint64
	OutNUcastPkts               uint64
	OutDiscards                 uint64
	OutErrors                   uint64
	OutUcastOctets              uint64
	OutMulticastOctets          uint64
	OutBroadcastOctets          uint64
	OutQLen                     uint64
}

type mibIfTable2 struct {
	NumEntries uint32
	_          uint32 // Padding
	Table      [1]mibIfRow2
}

// Processes

type processMemoryCounters struct {
	CB                         uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uintptr
	WorkingSetSize             uintptr // Resident Set Size
	QuotaPeakPagedPoolUsage    uintptr
	QuotaPagedPoolUsage        uintptr
	QuotaPeakNonPagesPoolUsage uintptr
	QuotaNonPagedPoolUsage     uintptr
	PagefileUsage              uintptr
	PeakPagefileUsage          uintptr
}

// WLAN

type wlanInterfaceInfoList struct {
	NumberOfItems uint32
	Index         uint32
	InterfaceInfo [1]wlanInterfaceInfo
}

type wlanInterfaceInfo struct {
	InterfaceGuid        windows.GUID
	InterfaceDescription [256]uint16
	IsState              uint32
}

type wlanConnectionAttributes struct {
	IsState                   uint32
	WlanConnectionMode        uint32
	StrProfileName            [256]uint16
	WlanAssociationAttributes wlanAssociationAttributes
	WlanSecurityAttributes    wlanSecurityAttributes
}

type wlanAssociationAttributes struct {
	Dot11Ssid         dot11Ssid
	Dot11BssType      uint32
	Dot11Bssid        [6]byte
	Dot11PhyType      uint32
	UDot11PhyIndex    uint32
	WlanSignalQuality uint32
	UlRxRate          uint32
	UlTxRate          uint32
}

type dot11Ssid struct {
	USSIDLength uint32
	UcSSID      [32]byte
}

type wlanSecurityAttributes struct {
	BSecurityEnabled     uint32
	BOneXEnabled         uint32
	Dot11AuthAlgorithm   uint32
	Dot11CipherAlgorithm uint32
}

// --- DLL & Procedure Handles

var (
	// DLLs

	iphlpapi = windows.NewLazySystemDLL("iphlpapi.dll")
	ntdll    = windows.NewLazySystemDLL("ntdll.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")
	wlanapi  = windows.NewLazySystemDLL("wlanapi.dll")
	psapi    = windows.NewLazySystemDLL("psapi.dll")

	// CPU/System

	procNtQuerySystemInformation = ntdll.NewProc("NtQuerySystemInformation")
	procGetNativeSystemInfo      = kernel32.NewProc("GetNativeSystemInfo")
	procGetTickCount64           = kernel32.NewProc("GetTickCount64")

	// Filesystem/Disk

	procGetLogicalDrives     = kernel32.NewProc("GetLogicalDrives")
	procGetDriveType         = kernel32.NewProc("GetDriveTypeW")
	procGetVolumeInformation = kernel32.NewProc("GetVolumeInformationW")
	procGetDiskFreeSpaceEx   = kernel32.NewProc("GetDiskFreeSpaceExW")

	// Memory

	procGlobalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")

	// Network

	procGetIfTable2  = iphlpapi.NewProc("GetIfTable2")
	procFreeMibTable = iphlpapi.NewProc("FreeMibTable")

	// Processes

	procGetProcessMemoryInfo = psapi.NewProc("GetProcessMemoryInfo")

	// WLAN

	wlanOpenHandle     = wlanapi.NewProc("WlanOpenHandle")
	wlanCloseHandle    = wlanapi.NewProc("WlanCloseHandle")
	wlanEnumInterfaces = wlanapi.NewProc("WlanEnumInterfaces")
	wlanQueryInterface = wlanapi.NewProc("WlanQueryInterface")
	wlanFreeMemory     = wlanapi.NewProc("WlanFreeMemory")
)
