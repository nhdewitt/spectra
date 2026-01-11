package collector

import (
	"net"
	"os"
	"runtime"

	"github.com/nhdewitt/spectra/internal/protocol"
)

const AgentVersion = "0.1.0"

func CollectHostInfo() protocol.HostInfo {
	platform, platVer := getPlatformInfo()

	return protocol.HostInfo{
		Hostname: getHostname(),
		OS:       runtime.GOOS,
		Platform: platform,
		PlatVer:  platVer,
		Kernel:   getKernel(),
		Arch:     runtime.GOARCH,
		CPUModel: getCPUModel(),
		CPUCores: runtime.NumCPU(),
		RAMTotal: getRAMTotal(),
		AgentVer: AgentVersion,
		BootTime: getBootTime(),
		IPs:      getIPs(),
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
