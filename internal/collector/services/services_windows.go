//go:build windows

package services

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"unsafe"

	"github.com/nhdewitt/spectra/internal/collector"
	"github.com/nhdewitt/spectra/internal/protocol"
	"github.com/nhdewitt/spectra/internal/winapi"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

// skipReason accumulates how many services failed for one distinct cause,
// plus one example of which service and which operation.
type skipReason struct {
	count   int
	example string
}

func MakeCollector(_ string) collector.CollectFunc {
	return Collect
}

func Collect(ctx context.Context) ([]protocol.Metric, error) {
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

	// A handful of protected services reject OpenService even for an admin,
	// so a skip is expected here, not a failure. Collection continues and the
	// skips are returned as one grouped error alongside a complete-as-possible
	// list -- Run logs the error and still forwards the data.
	var skipped int
	reasons := make(map[string]*skipReason)

	skip := func(name, op string, err error) {
		skipped++
		cause := err.Error()
		r, ok := reasons[cause]
		if !ok {
			r = &skipReason{example: fmt.Sprintf("%s %q", op, name)}
			reasons[cause] = r
		}
		r.count++
	}

	for _, name := range names {
		s, err := m.OpenService(name)
		if err != nil {
			skip(name, "opening service", err)
			continue
		}

		status, err := s.Query()
		if err != nil {
			s.Close()
			skip(name, "querying service", err)
			continue
		}

		cfg, err := s.Config()
		if err != nil {
			s.Close()
			skip(name, "reading config for service", err)
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

	skippedErr := summarizeSkips(skipped, len(names), reasons)

	return []protocol.Metric{
		protocol.ServiceListMetric{Services: services},
	}, skippedErr
}

// summarizeSkips renders grouped skip counts into a single error, or nil when
// nothing was sklipped. Causes are sorted so repeated samples produce a byte-identical
// message, and truncated past maxCauses so a host where every service is unreadable
// still logs one line.
func summarizeSkips(skipped, total int, reasons map[string]*skipReason) error {
	if skipped == 0 {
		return nil
	}

	const maxCauses = 3

	causes := make([]string, 0, len(reasons))
	for cause, r := range reasons {
		causes = append(causes, fmt.Sprintf("%dx %s (e.g. %s)", r.count, cause, r.example))
	}
	sort.Strings(causes)

	shown := causes
	suffix := ""
	if len(shown) > maxCauses {
		remaining := len(causes) - maxCauses
		shown = shown[:maxCauses]
		if remaining == 1 {
			suffix = "; and 1 other cause"
		} else {
			suffix = fmt.Sprintf("; and %d other causes", remaining)
		}
	}

	return fmt.Errorf("%d of %d services unreadable: %s%s",
		skipped, total, strings.Join(shown, "; "), suffix)
}

// getServiceDescription wraps QueryServiceConfig2W
func getServiceDescription(handle windows.Handle) string {
	var bytesNeeded uint32

	// First call - determine buffer size
	winapi.ProcQueryServiceConfig2W.Call(
		uintptr(handle),
		uintptr(winapi.ServiceConfigDescription),
		0,
		0,
		uintptr(unsafe.Pointer(&bytesNeeded)),
	)

	if bytesNeeded == 0 {
		return ""
	}

	buf := make([]byte, bytesNeeded)

	// Second call - retrieve data
	r1, _, _ := winapi.ProcQueryServiceConfig2W.Call(
		uintptr(handle),
		uintptr(winapi.ServiceConfigDescription),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(bytesNeeded),
		uintptr(unsafe.Pointer(&bytesNeeded)),
	)

	if r1 == 0 {
		return ""
	}

	// Cast the buffer to the serviceDescription struct
	descStruct := (*winapi.ServiceDescription)(unsafe.Pointer(&buf[0]))
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
