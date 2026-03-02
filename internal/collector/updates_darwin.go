//go:build darwin

package collector

import (
	"bufio"
	"bytes"
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

// CollectUpdates checks for pending macOS software updates.
//
// Uses softwareupdate -l which lists available system and
// security updates.
func CollectUpdates(ctx context.Context) ([]protocol.Metric, error) {
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "softwareupdate", "-l").Output()
	if err != nil {
		// Exit code 2 when no updates available
		if len(out) == 0 {
			return []protocol.Metric{protocol.UpdateMetric{
				PackageManager: "softwareupdate",
			}}, nil
		}
	}

	updates, rebootRequired := parseSoftwareUpdate(out)

	return []protocol.Metric{protocol.UpdateMetric{
		PendingCount:   len(updates),
		SecurityCount:  0, // Darwin does not easily distinguish security updates
		RebootRequired: rebootRequired,
		PackageManager: "softwareupdate",
		Packages:       updates,
	}}, nil
}

// parseSoftwareUpdate parses the output of softwareupdate -l.
//
// Example output:
//
// Software Update found the following new or updated software:
//   - Label: macOS Ventura 13.6.1
//     Title: macOS Ventura 13.6.1, Version: 13.6.1, SizE: 1024K, Recommended: YES, action: restart,
//   - Label: Security Update 2024-001
//     Title: Security Update 2024-001, Version: 1.0, Size: 512K, Recommnded: YES,
//
// If no updates are available:
// No new software available.
func parseSoftwareUpdate(data []byte) (updates []protocol.PendingUpdate, rebootRequired bool) {
	scanner := bufio.NewScanner(bytes.NewReader(data))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if !strings.HasPrefix(line, "* Label:") {
			continue
		}

		name := strings.TrimPrefix(line, "* Label:")
		name = strings.TrimSpace(name)

		// Parse the detail line that follows
		var version string
		if scanner.Scan() {
			detail := strings.TrimSpace(scanner.Text())
			version = extractField(detail, "Version:")
			if !rebootRequired && extractField(detail, "Action:") == "restart" {
				rebootRequired = true
			}
		}

		// Trim version suffix at the end of the name, since we have this
		// separately. Handle both "oldVer-newVer" and "newVer"
		if version != "" {
			name = strings.TrimSuffix(name, " "+version+"-"+version)
			name = strings.TrimSuffix(name, " "+version)
		}

		u := protocol.PendingUpdate{
			Name:    name,
			Version: version,
		}

		updates = append(updates, u)
	}

	return updates, rebootRequired
}

// extractField pulls a value from a comma-separated detail line.
// e.g. extractField("Title: foo, Version: 1.0, Size: 512K", "Version:") => "1.0"
func extractField(line, key string) string {
	idx := strings.Index(line, key)
	if idx == -1 {
		return ""
	}
	rest := line[idx+len(key):]
	if comma := strings.Index(rest, ","); comma != -1 {
		rest = rest[:comma]
	}
	return strings.TrimSpace(rest)
}
