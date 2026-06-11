package hostinfo

import (
	"net"
	"os"
	"runtime"

	"github.com/nhdewitt/spectra/internal/platform"
	"github.com/nhdewitt/spectra/internal/protocol"
	"github.com/nhdewitt/spectra/internal/version"
)

func CollectHostInfo() protocol.HostInfo {
	plat, platVer := getPlatformInfo()

	return protocol.HostInfo{
		Hostname: getHostname(),
		OS:       runtime.GOOS,
		Platform: plat,
		PlatVer:  platVer,
		Kernel:   getKernel(),
		Arch:     getArch(),
		CPUModel: getCPUModel(),
		CPUCores: runtime.NumCPU(),
		RAMTotal: getRAMTotal(),
		BootTime: getBootTime(),
		IPs:      getIPs(),
		Hardware: platform.HardwareClass(),
	}
}

func getHostname() string {
	h, _ := os.Hostname()
	return h
}

func getIPs() []string {
	var ips []string
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ips
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ips = append(ips, ipnet.IP.String())
			}
		}
	}

	return ips
}

func getArch() string {
	if runtime.GOARCH == "arm" && version.GoARM != "" {
		return "armv" + version.GoARM
	}
	return runtime.GOARCH
}
