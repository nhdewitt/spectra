//go:build darwin

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
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func getNetstat(ctx context.Context) ([]protocol.NetstatEntry, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var results []protocol.NetstatEntry

	for _, proto := range []string{"tcp", "udp"} {
		out, err := exec.CommandContext(ctx, "netstat", "-an", "p", proto).Output()
		if err != nil {
			continue
		}

		entries, err := parseNetstatFrom(bytes.NewReader(out), proto)
		if err != nil {
			continue
		}

		results = append(results, entries...)
	}

	return results, nil
}

// parseNetstatFrom parses Darwin netstat -an output.
//
// TCP:
//
// Proto	Recv-Q	Send-Q	Local Address		Foreign Address		(state)
// tcp4		     0	     0	192.168.1.10.443	10.0.0.1.52341		ESTABLISHED
// tcp46	     0	     0	*.443				*.*					LISTEN
//
// UDP:
//
// Proto	Recv-Q	Send-Q	Local Address		Foreign Address		(state)
// udp4		     0	     0	*.5353				*.*
//
// Addresses use dot separators for port, IPv6 uses bracketless notation.
func parseNetstatFrom(r io.Reader, protoFilter string) ([]protocol.NetstatEntry, error) {
	scanner := bufio.NewScanner(r)
	var results []protocol.NetstatEntry

	for scanner.Scan() {
		line := scanner.Text()

		// skip headers+non-data lines
		if !strings.HasPrefix(line, protoFilter) {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		proto := normalizeProto(fields[0])

		lAddr, lPort, err := parseAddr(fields[3])
		if err != nil {
			continue
		}

		rAddr, rPort, err := parseAddr(fields[4])
		if err != nil {
			continue
		}

		var state string
		if len(fields) >= 6 && protoFilter == "tcp" {
			state = fields[5]
		}

		// skip TCP entries without a state field to avoid duplicates
		if protoFilter == "tcp" && state == "" {
			continue
		}

		results = append(results, protocol.NetstatEntry{
			Proto:      proto,
			LocalAddr:  lAddr,
			LocalPort:  lPort,
			RemoteAddr: rAddr,
			RemotePort: rPort,
			State:      state,
		})
	}

	return results, scanner.Err()
}

// normalizeProto maps Darwin's proto field (tcp4, tcp6, tcp46,
// udp4, udp6, udp46) to standard names (tcp, tcp6, udp, udp6).
func normalizeProto(p string) string {
	switch p {
	case "tcp4":
		return "tcp"
	case "tcp6", "tcp46":
		return "tcp6"
	case "udp4":
		return "udp"
	case "udp6", "udp46":
		return "udp6"
	default:
		return p
	}
}

// parseAddr parses Darwin netstat address format.
//
// IPv4: 192.168.1.10.443 -> ("192.168.1.10", 443)
// IPv4 wildcard: "*.443" -> ("*", 443)
// IPv4 any: "*.*" -> ("*". 0)
// IPv6: "fe80::1%lo0.443" -> ("fe80::1%lo0", 443)
// IPv6: "::1.443" -> ("::1", 443)
//
// The port is always after the last dot.
func parseAddr(address string) (string, uint16, error) {
	if address == "*.*" {
		return "*", 0, nil
	}

	// find the last dot, separate addr, port
	lastDot := strings.LastIndex(address, ".")
	if lastDot == -1 {
		return "", 0, fmt.Errorf("no dot separator in %q", address)
	}

	addr := address[:lastDot]
	portStr := address[lastDot+1:]

	if portStr == "*" {
		return addr, 0, nil
	}

	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return "", 0, fmt.Errorf("bad port %q: %w", portStr, err)
	}

	return addr, uint16(port), nil
}
