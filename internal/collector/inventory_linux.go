//go:build linux

package collector

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/nhdewitt/spectra/internal/protocol"
)

// GetInstalledApps iterates through known package managers and uses the first one found.
func GetInstalledApps(ctx context.Context) ([]protocol.Application, error) {
	managers := []struct {
		name          string
		exe           string
		args          []string
		defaultVendor string
		separator     string
	}{
		{
			name:          "dpkg",
			exe:           "dpkg-query",
			args:          []string{"-W", "-f=${Package},${Version},${Maintainer}\n"},
			defaultVendor: "deb",
			separator:     ",",
		},
		{
			name:          "rpm",
			exe:           "rpm",
			args:          []string{"-qa", "--queryformat", "%{NAME},%{VERSION},%{VENDOR}\n"},
			defaultVendor: "rpm",
			separator:     ",",
		},
		{
			name:          "pacman",
			exe:           "pacman",
			args:          []string{"-Q"},
			defaultVendor: "arch",
			separator:     " ",
		},
		{
			name:          "apk",
			exe:           "apk",
			args:          []string{"info", "-v"},
			defaultVendor: "alpine",
			separator:     "-",
		},
	}

	for _, mgr := range managers {
		if _, err := exec.LookPath(mgr.exe); err != nil {
			continue
		}

		cmd := exec.CommandContext(ctx, mgr.exe, mgr.args...)
		out, err := cmd.StdoutPipe()
		if err != nil {
			return nil, fmt.Errorf("failed to pipe %s: %w", mgr.name, err)
		}

		if err := cmd.Start(); err != nil {
			return nil, fmt.Errorf("failed to run %s: %w", mgr.name, err)
		}

		apps, parseErr := parseGenericOutput(out, mgr.separator, mgr.defaultVendor)

		if waitErr := cmd.Wait(); waitErr != nil {
			if parseErr != nil {
				return nil, waitErr
			}
		}

		return apps, parseErr
	}

	return nil, nil // No supported package manager found
}

// parseGenericOutput handles simple name[sep]version lines
func parseGenericOutput(r io.Reader, separator, defaultVendor string) ([]protocol.Application, error) {
	var apps []protocol.Application
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Separate APK handling to avoid slice allocation
		if separator == "-" {
			if idx := findApkVersionIndex(line); idx != -1 {
				apps = append(apps, protocol.Application{
					Name:    line[:idx],
					Version: line[idx+1:],
					Vendor:  defaultVendor,
				})
			}
			continue
		}

		var parts []string
		var vendor string = defaultVendor

		switch separator {
		case " ":
			// Pacman: name vendor
			parts = strings.Fields(line)
			if len(parts) > 2 {
				parts = []string{parts[0], parts[1]}
			}
		case ",":
			// DPKG/RPM: name,version,vendor
			parts = strings.SplitN(line, separator, 3)
			if len(parts) == 3 {
				if cleaned := cleanVendor(parts[2]); cleaned != "" {
					vendor = cleaned
				}
			}
		}

		if len(parts) >= 2 && parts[0] != "" && parts[1] != "" {
			apps = append(apps, protocol.Application{
				Name:    parts[0],
				Version: parts[1],
				Vendor:  vendor,
			})
		}
	}

	return apps, scanner.Err()
}

// findApkVersionIndex returns the index where the version starts or -1
func findApkVersionIndex(line string) int {
	// Scan backwards for the first '-' followed by a digit
	for i := len(line) - 2; i > 0; i-- {
		if line[i] == '-' && isDigit(line[i+1]) {
			return i
		}
	}
	return -1
}
