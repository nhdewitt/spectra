//go:build windows

package diagnostics

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os/exec"
	"strconv"
	"strings"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func getNetstat(ctx context.Context) ([]protocol.NetstatEntry, error) {
	cmd := exec.CommandContext(ctx, "netstat", "-ano")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("netstat command failed: %w", err)
	}

	return parseNetstatFrom(bytes.NewReader(out))
}

func parseNetstatFrom(r io.Reader) ([]protocol.NetstatEntry, error) {
	var entries []protocol.NetstatEntry
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		proto := strings.ToLower(fields[0])
		if proto != "tcp" && proto != "udp" {
			continue
		}

		lAddr, lPort, err := splitHostPort(fields[1])
		if err != nil {
			continue
		}
		rAddr, rPort, err := splitHostPort(fields[2])
		if err != nil {
			continue
		}

		entry := protocol.NetstatEntry{
			Proto:      proto,
			LocalAddr:  lAddr,
			LocalPort:  lPort,
			RemoteAddr: rAddr,
			RemotePort: rPort,
		}

		var pid uint64
		var parseErr error

		if proto == "tcp" && len(fields) >= 5 {
			entry.State = fields[3]
			pid, parseErr = strconv.ParseUint(fields[4], 10, 32)
		} else {
			entry.State = ""
			pid, parseErr = strconv.ParseUint(fields[3], 10, 32)
		}

		if parseErr != nil {
			continue
		}
		entry.PID = uint32(pid)

		entries = append(entries, entry)
	}

	return entries, scanner.Err()
}

func splitHostPort(val string) (string, uint16, error) {
	if val == "*:*" {
		return "0.0.0.0", 0, nil
	}

	host, portStr, err := net.SplitHostPort(val)
	if err != nil {
		return "", 0, fmt.Errorf("bad format: %w", err)
	}

	if portStr == "*" {
		return host, 0, nil
	}

	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return "", 0, fmt.Errorf("bad port: %w", err)
	}

	return host, uint16(port), nil
}
