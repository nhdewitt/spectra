//go:build !windows

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

var statusIntern = map[string]string{
	"loaded":    "loaded",
	"not-found": "not-found",
	"active":    "active",
	"inactive":  "inactive",
	"running":   "running",
	"dead":      "dead",
	"failed":    "failed",
	"exited":    "exited",
}

func CollectServices(ctx context.Context) ([]protocol.Metric, error) {
	cmd := exec.CommandContext(ctx,
		"systemctl", "list-units",
		"--type=service", "--all",
		"--no-pager", "--no-legend",
		"--plain",
	)

	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	return parseSystemctlFrom(bytes.NewReader(out))
}

func parseSystemctlFrom(r io.Reader) ([]protocol.Metric, error) {
	services := make([]protocol.ServiceMetric, 0, 128)
	scanner := bufio.NewScanner(r)

	var descriptionBuilder strings.Builder
	descriptionBuilder.Grow(64)

	for scanner.Scan() {
		descriptionBuilder.Reset()

		line := scanner.Bytes()
		fields := bytes.Fields(line)

		if len(fields) < 4 {
			continue
		}

		if bytes.HasPrefix(fields[0], []byte("snap-")) || bytes.HasPrefix(fields[0], []byte("dev-loop")) {
			continue
		}

		for i, field := range fields[4:] {
			if i > 0 {
				descriptionBuilder.WriteByte(' ')
			}
			descriptionBuilder.Write(field)
		}

		services = append(services, protocol.ServiceMetric{
			Name:        string(fields[0]),
			LoadState:   intern(fields[1]),
			Status:      string(fields[2]),
			SubStatus:   string(fields[3]),
			Description: descriptionBuilder.String(),
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return []protocol.Metric{
		&protocol.ServiceListMetric{Services: services},
	}, nil
}

func intern(b []byte) string {
	if s, ok := statusIntern[string(b)]; ok {
		return s
	}
	return string(b)
}
