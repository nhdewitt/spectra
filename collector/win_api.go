//go:build windows

package collector

import (
	"golang.org/x/sys/windows"
)

// --- DLL Handles ---
var (
	iphlpapi = windows.NewLazySystemDLL("iphlpapi.dll")
	ntdll    = windows.NewLazySystemDLL("ntdll.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")
	wlanapi  = windows.NewLazySystemDLL("wlanapi.dll")
)

// --- Procedure Handles ---
var (
	// CPU/System

	procNtQuerySystemInformation = ntdll.NewProc("NtQuerySystemInformation")
	procGetNativeSystemInfo      = kernel32.NewProc("GetNativeSystemInfo")

	// Filesystem/Disk usage

	procGetLogicalDrives     = kernel32.NewProc("GetLogicalDrives")
	procGetDriveType         = kernel32.NewProc("GetDriveTypeW")
	procGetVolumeInformation = kernel32.NewProc("GetVolumeInformationW")
	procGetDiskFreeSpaceEx   = kernel32.NewProc("GetDiskFreeSpaceExW")

	// Memory

	procGlobalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")

	// Network

	procGetIfTable2  = iphlpapi.NewProc("GetIfTable2")
	procFreeMibTable = iphlpapi.NewProc("FreeMibTable")

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

	// Disk

	// Drive Types

	driveUnknown   = 0
	driveNoRootDir = 1
	driveRemovable = 2
	driveFixed     = 3
	driveRemote    = 4
	driveCdrom     = 5
	driveRamdisk   = 6

	// Disk IO

	// IOCTL_DISK_PERFORMANCE: 0x70020
	// ControlCode(DeviceType: 7 (Disk), Function: 8, Method: 0 (Buffered), Access: 0 (Any))
	ioctlDiskPerformance = 0x70020
	// IOCTL_STORAGE_QUERY_PROPERTY: 0x2D1400
	ioctlStorageQueryProperty = 0x2D1400
	// IOCTL_VOLUME_GET_VOLUME_DISK_EXTENTS: 0x560000
	ioctlVolumeGetVolumeDiskExtents = 0x560000

	// Storage Property Types

	// PropertyId
	storageDeviceProperty = 0
	// QueryType
	propertyStandardQuery = 0

	// Bus Types

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

	// WLAN_API_VERSION_2_0 is for Windows Vista+
	wlanApiVersion = 2

	// OpCodes from wlanapi.h WLAN_INTF_OPCODE enum

	wlanIntfOpcodeCurrentConnection = 0x00000007
	wlanIntfOpcodeChannelNumber     = 0x00000008
	wlanIntfOpcodeChannelFrequency  = 0x00000067 // Windows 10+
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

// Disk: STORAGE_PROPERTY_QUERY
type storagePropertyQuery struct {
	PropertyId           uint32
	QueryType            uint32
	AdditionalParameters [1]byte
}

// Disk: STORAGE_DEVICE_DESCRIPTOR
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

// Disk: VOLUME_DISK_EXTENTS
type diskExtent struct {
	DiskNumber     uint32
	StartingOffset int64
	ExtentLength   int64
}

type volumeDiskExtents struct {
	NumberOfDiskExtents uint32
	Extents             [1]diskExtent
}

// Disk IO: Disk Performance
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

// Network: MIB_IF_ROW2
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

// Network: MIB_IF_TABLE2
type mibIfTable2 struct {
	NumEntries uint32
	_          uint32 // Padding
	Table      [1]mibIfRow2
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
