package platform

type PackageManager int

const (
	PkgNone PackageManager = iota
	PkgApt
	PkgYum
	PkgApk
	PkgPacman
	PkgWindowsUpdate
)

type InitSystem int

const (
	InitUnknown InitSystem = iota
	InitSystemd
	InitOpenRC
	InitSysV
)

type Info struct {
	// Package management
	PackageManager PackageManager
	PackageExe     string

	// Init system
	InitSystem    InitSystem
	SystemctlPath string

	// Kernel/OS
	HasPSI        bool
	CgroupVersion int // v1 or v2
	KernelVersion string

	// Hardware
	NumCPU        int
	IsRaspberryPi bool
	PiModel       string
	VcgencmdPath  string

	// Thermal
	ThermalZones []string

	// Tools
	SmartctlPath   string
	PowerShellPath string
}
