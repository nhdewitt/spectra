//go:build darwin

package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

type systemProfilerOutput struct {
	Apps []systemProfilerApp `json:"SPApplicationsDataType"`
}

type systemProfilerApp struct {
	Name         string `json:"_name"`
	Version      string `json:"version"`
	ObtainedFrom string `json:"obtained_from"`
	Path         string `json:"path"`
}

var returnedWarning bool

const systemProfilerTimeout = 60 * time.Second

// GetInstalledApps returns all applications found by system_profiler.
// JSON support for system_profiler was added in macOS Catalina (10.15).
// Previous versions will return an empty list.
//
// Also returns an empty list if Spotlight is disabled or if system_profiler
// fails or times out.
func GetInstalledApps(ctx context.Context) ([]protocol.Application, error) {
	ctx, cancel := context.WithTimeout(ctx, systemProfilerTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, "system_profiler", "SPApplicationsDataType", "-json").Output()
	if err != nil {
		if !returnedWarning {
			log.Printf("inventory: system_profiler failed: %v", err)
			returnedWarning = true
		}
		return []protocol.Application{}, err
	}

	apps, err := parseSystemProfiler(out)
	if err != nil {
		if !returnedWarning {
			log.Printf("inventory: failed to parse system_profiler output: %v", err)
			returnedWarning = true
		}
		return []protocol.Application{}, err
	}
	if len(apps) == 0 {
		if !returnedWarning {
			log.Print("inventory: system_profiler returned 0 apps (Spotlight possibly disabled).")
			returnedWarning = true
		}
		return apps, nil
	}

	returnedWarning = false
	return apps, nil
}

func parseSystemProfiler(data []byte) ([]protocol.Application, error) {
	var sp systemProfilerOutput
	if err := json.Unmarshal(data, &sp); err != nil {
		return nil, fmt.Errorf("parsing system_profiler output: %w", err)
	}

	apps := make([]protocol.Application, 0, len(sp.Apps))
	for _, a := range sp.Apps {
		apps = append(apps, protocol.Application{
			Name:    a.Name,
			Version: a.Version,
			Vendor:  normalizeVendor(a.ObtainedFrom),
		})
	}

	return apps, nil
}

func normalizeVendor(obtainedFrom string) string {
	switch strings.ToLower(obtainedFrom) {
	case "apple":
		return "Apple"
	case "mac_app_store":
		return "Mac App Store"
	case "identified_developer":
		return "Identified Developer"
	case "unknown":
		return "Unknown"
	default:
		return obtainedFrom
	}
}
