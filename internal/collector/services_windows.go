//go:build windows

package collector

import (
	"context"
	"fmt"
	"unsafe"

	"github.com/nhdewitt/spectra/internal/protocol"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

type winService struct {
	Name        string `json:"Name"`
	DisplayName string `json:"DisplayName"`
	State       string `json:"State"`
	StartMode   string `json:"StartMode"`
	Description string `json:"Description"`
}

func CollectServices(ctx context.Context) ([]protocol.Metric, error) {
	m, err := mgr.Connect()
	if err != nil {
		return nil, fmt.Errorf("SCM connection failed: %w", err)
	}
	defer m.Disconnect()

	names, err := m.ListServices()
	if err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}

	services := make([]protocol.ServiceMetric, 0, len(names))

	for _, name := range names {
		s, err := m.OpenService(name)
		if err != nil {
			continue
		}

		status, err := s.Query()
		if err != nil {
			s.Close()
			continue
		}

		cfg, err := s.Config()
		if err != nil {
			s.Close()
			continue
		}

		// Get the full text description
		fullDesc := getServiceDescription(s.Handle)
		s.Close()

		descriptionText := cfg.DisplayName
		if fullDesc != "" && fullDesc != cfg.DisplayName {
			descriptionText = fmt.Sprintf("%s - %s", cfg.DisplayName, fullDesc)
		}

		loadState := "loaded"
		if cfg.StartType == mgr.StartDisabled {
			loadState = "disabled"
		}

		services = append(services, protocol.ServiceMetric{
			Name:        name,
			Status:      mapState(status.State),
			SubStatus:   mapStartType(cfg.StartType),
			LoadState:   loadState,
			Description: descriptionText,
		})
	}

	return []protocol.Metric{
		protocol.ServiceListMetric{Services: services},
	}, nil
}

// getServiceDescription wraps QueryServiceConfig2W
func getServiceDescription(handle windows.Handle) string {
	var bytesNeeded uint32

	// First call - determine buffer size
	procQueryServiceConfig2W.Call(
		uintptr(handle),
		uintptr(serviceConfigDescription),
		0,
		0,
		uintptr(unsafe.Pointer(&bytesNeeded)),
	)

	if bytesNeeded == 0 {
		return ""
	}

	buf := make([]byte, bytesNeeded)

	// Second call - retrieve data
	r1, _, _ := procQueryServiceConfig2W.Call(
		uintptr(handle),
		uintptr(serviceConfigDescription),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(bytesNeeded),
		uintptr(unsafe.Pointer(&bytesNeeded)),
	)

	if r1 == 0 {
		return ""
	}

	// Cast the buffer to the serviceDescription struct
	descStruct := (*serviceDescription)(unsafe.Pointer(&buf[0]))
	if descStruct.Description == nil {
		return ""
	}

	return windows.UTF16PtrToString(descStruct.Description)
}

// mapState returns the string of the service execution state.
func mapState(s svc.State) string {
	switch s {
	case svc.Stopped:
		return "Stopped"
	case svc.StartPending:
		return "StartPending"
	case svc.StopPending:
		return "StopPending"
	case svc.Running:
		return "Running"
	case svc.ContinuePending:
		return "ContinuePending"
	case svc.PausePending:
		return "PausePending"
	case svc.Paused:
		return "Paused"
	default:
		return "Unknown"
	}
}

func mapStartType(startType uint32) string {
	switch startType {
	case windows.SERVICE_BOOT_START:
		return "Boot"
	case windows.SERVICE_SYSTEM_START:
		return "System"
	case windows.SERVICE_AUTO_START:
		return "Auto"
	case windows.SERVICE_DEMAND_START:
		return "Manual"
	case windows.SERVICE_DISABLED:
		return "Disabled"
	default:
		return "Unknown"
	}
}
