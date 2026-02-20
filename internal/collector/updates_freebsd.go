//go:build freebsd

package collector

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os/exec"
	"strings"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func CollectUpdates(ctx context.Context) ([]protocol.Metric, error) {
	out, err := exec.CommandContext(ctx, "pkg", "upgrade", "-n").Output()
	if err != nil {
		if len(out) == 0 {
			return nil, err
		}
	}

	updates, err := parsePkgUpgrade(bytes.NewReader(out))
	if err != nil {
		return nil, err
	}

	return []protocol.Metric{protocol.UpdateMetric{
		PendingCount:   len(updates),
		SecurityCount:  0, // pkg doesn't distinguish security updates
		RebootRequired: false,
		PackageManager: "pkg",
		Packages:       updates,
	}}, nil
}

func parsePkgUpgrade(r io.Reader) ([]protocol.PendingUpdate, error) {
	scanner := bufio.NewScanner(r)
	var updates []protocol.PendingUpdate
	inUpgradeSection := false

	for scanner.Scan() {
		raw := scanner.Text()
		line := strings.TrimSpace(raw)

		if !inUpgradeSection {
			if strings.Contains(line, "Installed packages to be UPGRADED:") {
				inUpgradeSection = true
			}
			continue
		}

		if line == "" {
			continue
		}

		// Anything unindented or empty means we've reached the end of the upgrade section
		if !strings.HasPrefix(raw, "\t") && !strings.HasPrefix(raw, "        ") {
			break
		}

		// Format: "<program>: <oldversion> -> <newversion> [repo]"
		u, ok := parsePkgUpgradeLine(line)
		if ok {
			updates = append(updates, u)
		}
	}

	return updates, scanner.Err()
}

// parsePkgUpgradeLine parses "program: old_version -> new_version [repo]".
func parsePkgUpgradeLine(line string) (protocol.PendingUpdate, bool) {
	name, rest, ok := strings.Cut(line, ":")
	if !ok {
		return protocol.PendingUpdate{}, false
	}

	fields := strings.Fields(rest)
	if len(fields) < 3 || fields[1] != "->" {
		return protocol.PendingUpdate{}, false
	}

	version := fields[2]

	return protocol.PendingUpdate{
		Name:    strings.TrimSpace(name),
		Version: version,
	}, true
}
