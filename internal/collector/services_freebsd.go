//go:build freebsd

package collector

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/nhdewitt/spectra/internal/protocol"
)

// CollectServices calls "service -e" to get enabled services
// and "service -l" to get all services, using the set of enabled
// services to determine which ones are currently active. FreeBSD
// collection does not include the substatus of services.
func CollectServices(ctx context.Context) ([]protocol.Metric, error) {
	enabledOut, err := exec.CommandContext(ctx, "service", "-e").Output()
	if err != nil {
		return nil, err
	}
	allOut, err := exec.CommandContext(ctx, "service", "-l").Output()
	if err != nil {
		return nil, err
	}

	enabledSet := parseEnabled(bytes.NewReader(enabledOut))
	allServices := parseAll(bytes.NewReader(allOut))

	services := make([]protocol.ServiceMetric, 0, len(allServices))
	for _, name := range allServices {
		loadState, status := isEnabled(name, enabledSet)

		services = append(services, protocol.ServiceMetric{
			Name:      name,
			LoadState: loadState,
			Status:    status,
		})
	}

	return []protocol.Metric{
		protocol.ServiceListMetric{Services: services},
	}, nil
}

func parseEnabled(r io.Reader) map[string]bool {
	set := make(map[string]bool)
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			set[filepath.Base(line)] = true
		}
	}

	return set
}

func parseAll(r io.Reader) []string {
	var names []string
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		name := strings.TrimSpace(scanner.Text())
		if name != "" {
			names = append(names, name)
		}
	}

	return names
}

func isEnabled(service string, serviceSet map[string]bool) (string, string) {
	if serviceSet[service] {
		return "enabled", "active"
	}
	return "disabled", "inactive"
}
