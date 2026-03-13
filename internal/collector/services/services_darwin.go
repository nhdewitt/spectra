//go:build darwin

package services

import (
	"bufio"
	"bytes"
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/nhdewitt/spectra/internal/collector"
	"github.com/nhdewitt/spectra/internal/protocol"
)

func MakeCollector(launchctlPath string) collector.CollectFunc {
	return func(ctx context.Context) ([]protocol.Metric, error) {
		if launchctlPath == "" {
			return nil, nil
		}

		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		out, err := exec.CommandContext(ctx, launchctlPath, "list").Output()
		if err != nil {
			return nil, err
		}

		return parseLaunchctlList(out)
	}
}

// parseLaunchctlList parses the output of 'launchctl list'.
//
// Format:
//
// PID	Status	Label
// -	0		com.apple...
// 23	-9		com.apple...
//
// PID: "-" if not running, numeric if running.
// Status: Last exit code (0 if clean).
func parseLaunchctlList(data []byte) ([]protocol.Metric, error) {
	services := make([]protocol.ServiceMetric, 0, 128)
	scanner := bufio.NewScanner(bytes.NewReader(data))

	// Skip header
	if !scanner.Scan() {
		return nil, scanner.Err()
	}

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// Tab-delimited
		fields := strings.SplitN(line, "\t", 3)
		if len(fields) < 3 {
			continue
		}

		pid := strings.TrimSpace(fields[0])
		exitStatus := strings.TrimSpace(fields[1])
		label := strings.TrimSpace(fields[2])

		if label == "" {
			continue
		}

		// Skip Apple-internal background items
		if strings.HasPrefix(label, "application.") {
			continue
		}

		status := "stopped"
		if pid != "-" && pid != "" {
			status = "running"
		}

		subStatus := "exited"
		if status == "running" {
			subStatus = "running"
		} else if exitStatus != "0" && exitStatus != "" {
			subStatus = "failed"
		}

		services = append(services, protocol.ServiceMetric{
			Name:      label,
			LoadState: "loaded",
			Status:    status,
			SubStatus: subStatus,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return []protocol.Metric{protocol.ServiceListMetric{
		Services: services,
	}}, nil
}
