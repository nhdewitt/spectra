//go:build darwin

package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

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

// GetInstalledApps returns all applications found by system_profiler.
func GetInstalledApps(ctx context.Context) ([]protocol.Application, error) {
	out, err := exec.CommandContext(ctx, "system_profiler", "SPApplicationsDataType", "-json").Output()
	if err != nil {
		return nil, err
	}

	return parseSystemProfiler(out)
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
