//go:build windows

package winapi

import (
	"golang.org/x/sys/windows"
)

// BusType represents the hardware interface (USB, SATA, NVMe, etc.)
type BusType uint32

type ProcessState uint32

const (
	// CPU/System

	SystemProcessorPerformanceInformation = 8

	// Disk: Drive Types

	driveUnknown   = 0
	driveNoRootDir = 1
	DriveRemovable = 2
	DriveFixed     = 3
	driveRemote    = 4
	driveCdrom     = 5
	driveRamdisk   = 6

	// Disk: IOCTL Codes

	// DeviceType: 7 (Disk), Function: 8, Method: 0 (Buffered), Access: 0 (Any)
	IoctlDiskPerformance            = 0x70020
	IoctlStorageQueryProperty       = 0x2D1400
	IoctlVolumeGetVolumeDiskExtents = 0x560000

	// Disk: Property Types

	StorageDeviceProperty = 0 // PropertyId
	StorageStandardQuery  = 0 // QueryType

	// Disk: Bus Types

	busTypeUnknown           BusType = 0x00
	busTypeScsi              BusType = 0x01
	busTypeAtapi             BusType = 0x02
	busTypeAta               BusType = 0x03
	BusType1394              BusType = 0x04
	busTypeSsa               BusType = 0x05
	busTypeFibre             BusType = 0x06
	BusTypeUsb               BusType = 0x07
	busTypeRAID              BusType = 0x08
	busTypeiScsi             BusType = 0x09
	busTypeSas               BusType = 0x0A
	busTypeSata              BusType = 0x0B
	busTypeSd                BusType = 0x0C
	busTypeMmc               BusType = 0x0D
	busTypeVirtual           BusType = 0x0E
	busTypeFileBackedVirtual BusType = 0x0F
	busTypeNvme              BusType = 0x11

	// Processes

	StateInitialized             ProcessState = 0
	StateReady                   ProcessState = 1
	StateRunning                 ProcessState = 2
	StateStandby                 ProcessState = 3
	StateTerminated              ProcessState = 4
	StateWaiting                 ProcessState = 5
	StateTransition              ProcessState = 6
	StateDeferredReady           ProcessState = 7
	StateGateWaitObsolete        ProcessState = 8
	StateWaitingForProcessInSwap ProcessState = 9

	StatusInfoLengthMismatch = 0xC0000004

	// Services

	// Service Config Levels
	ServiceConfigDescription = 1

	// WLAN

	WlanApiVersion = 2 // Windows Vista+

	// WLAN_INTF_OPCODE enum

	WlanIntfOpcodeCurrentConnection = 0x00000007
	WlanIntfOpcodeChannelNumber     = 0x00000008
	WlanIntfOpcodeChannelFrequency  = 0x00000067 // Windows 10+
	WlanIntfOpcodeRssi              = 0x10000102
)

// --- Struct Definitions ---

// CPU

// Native internal structure for NtQuerySystemInformation
type SystemProcessorPerformanceInfo struct {
	IdleTime       int64
	KernelTime     int64
	UserTime       int64
	DpcTime        int64
	InterruptTime  int64
	InterruptCount uint32
	_              uint32 // Padding for 64-bit alignment
}

// Hardware topology
type SystemInfo struct {
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

type StoragePropertyQuery struct {
	PropertyId           uint32
	QueryType            uint32
	AdditionalParameters [1]byte
}

type StorageDeviceDescriptor struct {
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

type DiskExtent struct {
	DiskNumber     uint32
	StartingOffset int64
	ExtentLength   int64
}

type VolumeDiskExtents struct {
	NumberOfDiskExtents uint32
	Extents             [1]DiskExtent
}

type DiskPerformance struct {
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

type MemoryStatusEx struct {
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

type MibIfRow2 struct {
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

type MibIfTable2 struct {
	NumEntries uint32
	_          uint32 // Padding
	Table      [1]MibIfRow2
}

// Processes

type ProcessMemoryCounters struct {
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

type SystemProcessInformation struct {
	NextEntryOffset              uint32
	NumberOfThreads              uint32
	WorkingSetPrivateSize        int64
	HardFaultCount               uint32
	NumberOfThreadsHighWatermark uint32
	CycleTime                    uint64
	CreateTime                   int64
	UserTime                     int64
	KernelTime                   int64
	ImageName                    unicodeString
	BasePriority                 int32
	_                            uint32 // Padding
	UniqueProcessId              uintptr
	InheritedFromUniqueProcessId uintptr
	HandleCount                  uint32
	SessionId                    uint32
	UniqueProcessKey             uintptr
	PeakVirtualSize              uintptr
	VirtualSize                  uintptr
	PageFaultCount               uint32
	PeakWorkingSetSize           uintptr
	WorkingSetSize               uintptr
	QuotaPeakPagedPoolUsage      uintptr
	QuotaPagedPoolUsage          uintptr
	QuotaPeakNonPagedPoolUsage   uintptr
	QuotaNonPagedPoolUsage       uintptr
	PagefileUsage                uintptr
	PeakPagefileUsage            uintptr
	PrivatePageCount             uintptr
	ReadOperationCount           int64
	WriteOperationCount          int64
	OtherOperationCount          int64
	ReadTransferCount            int64
	WriteTransferCount           int64
	OtherTransferCount           int64
}

type unicodeString struct {
	Length        uint16
	MaximumLength uint16
	_             [4]byte // Padding
	Buffer        uintptr
}

type SystemThreadInformation struct {
	KernelTime   int64
	UserTime     int64
	CreateTime   int64
	WaitTime     uint32
	_            uint32 // Padding
	StartAddress uintptr
	ClientId     struct {
		UniqueProcess uintptr
		UniqueThread  uintptr
	}
	Priority        int32
	BasePriority    int32
	ContextSwitches uint32
	ThreadState     ProcessState
	WaitReason      uint32
}

// Services

// https://learn.microsoft.com/en-us/windows/win32/api/winsvc/ns-winsvc-service_descriptiona
type ServiceDescription struct {
	Description *uint16
}

// WLAN

type WlanInterfaceInfoList struct {
	NumberOfItems uint32
	Index         uint32
	InterfaceInfo [1]wlanInterfaceInfo
}

type wlanInterfaceInfo struct {
	InterfaceGuid        windows.GUID
	InterfaceDescription [256]uint16
	IsState              uint32
}

type WlanConnectionAttributes struct {
	IsState                   uint32
	WlanConnectionMode        uint32
	StrProfileName            [256]uint16
	WlanAssociationAttributes wlanAssociationAttributes
	WlanSecurityAttributes    wlanSecurityAttributes
}

type wlanAssociationAttributes struct {
	Dot11Ssid         Dot11Ssid
	Dot11BssType      uint32
	Dot11Bssid        [6]byte
	Dot11PhyType      uint32
	UDot11PhyIndex    uint32
	WlanSignalQuality uint32
	UlRxRate          uint32
	UlTxRate          uint32
}

type Dot11Ssid struct {
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

	Advapi32 = windows.NewLazySystemDLL("advapi32.dll")
	Iphlpapi = windows.NewLazySystemDLL("iphlpapi.dll")
	Ntdll    = windows.NewLazySystemDLL("ntdll.dll")
	Kernel32 = windows.NewLazySystemDLL("kernel32.dll")
	Wlanapi  = windows.NewLazySystemDLL("wlanapi.dll")
	Psapi    = windows.NewLazySystemDLL("psapi.dll")

	// CPU/System

	ProcNtQuerySystemInformation = Ntdll.NewProc("NtQuerySystemInformation")
	ProcGetNativeSystemInfo      = Kernel32.NewProc("GetNativeSystemInfo")
	ProcGetTickCount64           = Kernel32.NewProc("GetTickCount64")

	// Filesystem/Disk

	ProcGetLogicalDrives     = Kernel32.NewProc("GetLogicalDrives")
	ProcGetDriveType         = Kernel32.NewProc("GetDriveTypeW")
	ProcGetVolumeInformation = Kernel32.NewProc("GetVolumeInformationW")
	ProcGetDiskFreeSpaceEx   = Kernel32.NewProc("GetDiskFreeSpaceExW")

	// Memory

	ProcGlobalMemoryStatusEx = Kernel32.NewProc("GlobalMemoryStatusEx")

	// Network

	ProcGetIfTable2  = Iphlpapi.NewProc("GetIfTable2")
	ProcFreeMibTable = Iphlpapi.NewProc("FreeMibTable")

	// Processes

	ProcGetProcessMemoryInfo = Psapi.NewProc("GetProcessMemoryInfo")

	// Services

	ProcQueryServiceConfig2W = Advapi32.NewProc("QueryServiceConfig2W")

	// WLAN

	WlanOpenHandle     = Wlanapi.NewProc("WlanOpenHandle")
	WlanCloseHandle    = Wlanapi.NewProc("WlanCloseHandle")
	WlanEnumInterfaces = Wlanapi.NewProc("WlanEnumInterfaces")
	WlanQueryInterface = Wlanapi.NewProc("WlanQueryInterface")
	WlanFreeMemory     = Wlanapi.NewProc("WlanFreeMemory")
)
