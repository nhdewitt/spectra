//go:build freebsd

package diagnostics

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func getNetstat(ctx context.Context) ([]protocol.NetstatEntry, error) {
	out, err := exec.CommandContext(ctx, "sockstat", "-s", "-46").Output()
	if err != nil {
		return nil, err
	}

	return getNetstatFrom(bytes.NewReader(out))
}

func getNetstatFrom(r io.Reader) ([]protocol.NetstatEntry, error) {
	scanner := bufio.NewScanner(r)
	var results []protocol.NetstatEntry

	// skip header
	if !scanner.Scan() {
		return nil, scanner.Err()
	}

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 8 {
			continue
		}

		user := fields[0]
		proto := fields[4]

		lAddr, lPort, err := parseAddr(fields[5])
		if err != nil {
			continue
		}

		rAddr, rPort, err := parseAddr(fields[6])
		if err != nil {
			continue
		}

		state := fields[7]
		if state == "??" {
			state = ""
		}

		results = append(results, protocol.NetstatEntry{
			Proto:      proto,
			LocalAddr:  lAddr,
			LocalPort:  lPort,
			RemoteAddr: rAddr,
			RemotePort: rPort,
			State:      state,
			User:       user,
		})
	}

	return results, scanner.Err()
}

// parseAddr splits "addr:port" on the last colon to handle IPv6 addresses.
func parseAddr(address string) (string, uint16, error) {
	if address == "*.*" {
		return "*", 0, nil
	}

	idx := strings.LastIndex(address, ":")
	if idx == -1 {
		return "", 0, fmt.Errorf("invalid format")
	}

	addr := address[:idx]
	portStr := address[idx+1:]
	if portStr == "*" {
		return addr, 0, nil
	}

	p, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return "", 0, err
	}

	return addr, uint16(p), nil
}
