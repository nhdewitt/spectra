//go:build !windows

package diagnostics

import (
	"bufio"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/nhdewitt/spectra/internal/protocol"
)

// Linux kernel TCP states
var tcpStates = map[string]string{
	"01": "ESTABLISHED",
	"02": "SYN_SENT",
	"03": "SYN_RECV",
	"04": "FIN_WAIT1",
	"05": "FIN_WAIT2",
	"06": "TIME_WAIT",
	"07": "CLOSE",
	"08": "CLOSE_WAIT",
	"09": "LAST_ACK",
	"0A": "LISTEN",
	"0B": "CLOSING",
}

func getNetstat(ctx context.Context) ([]protocol.NetstatEntry, error) {
	var results []protocol.NetstatEntry

	files := []struct {
		path     string
		proto    string
		required bool
	}{
		{"/proc/net/tcp", "tcp", true},
		{"/proc/net/udp", "udp", true},
		{"/proc/net/tcp6", "tcp6", false},
		{"/proc/net/udp6", "udp6", false},
	}

	for _, f := range files {
		if ctx.Err() != nil {
			return results, ctx.Err()
		}

		res, err := parseProcNet(f.path, f.proto)
		if err != nil && f.required {
			return nil, fmt.Errorf("failed to read %s: %w", f.proto, err)
		}
		results = append(results, res...)
	}

	return results, nil
}

// parseProcNet handles the OS file interaction
func parseProcNet(path, proto string) ([]protocol.NetstatEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return parseProcNetFrom(f, proto)
}

func parseProcNetFrom(r io.Reader, proto string) ([]protocol.NetstatEntry, error) {
	var entries []protocol.NetstatEntry
	scanner := bufio.NewScanner(r)

	// Skip header
	scanner.Scan()

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 10 {
			continue
		}

		lAddr, lPort, err := parseAddr(fields[1])
		if err != nil {
			continue
		}
		rAddr, rPort, err := parseAddr(fields[2])
		if err != nil {
			continue
		}

		stateHex := fields[3]
		uid := fields[7]

		state := tcpStates[stateHex]
		if state == "" {
			state = "UNKNOWN"
		}
		if strings.HasPrefix(proto, "udp") {
			state = ""
		}

		entries = append(entries, protocol.NetstatEntry{
			Proto:      proto,
			LocalAddr:  lAddr,
			LocalPort:  lPort,
			RemoteAddr: rAddr,
			RemotePort: rPort,
			State:      state,
			User:       uid,
		})
	}

	return entries, scanner.Err()
}

// parseAddr parses hex -> IP, Port (0100007F:1F90->127.0.0.1:8080)
func parseAddr(addr string) (string, uint16, error) {
	parts := strings.Split(addr, ":")
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("invalid format")
	}

	ipHex, portHex := parts[0], parts[1]

	var ip net.IP
	var err error
	switch len(ipHex) {
	case 8:
		ip, err = parseIPv4Hex(ipHex)
	case 32:
		ip, err = parseIPv6Hex(ipHex)
	default:
		return "", 0, fmt.Errorf("bad ip length: %d", len(ipHex))
	}

	if err != nil {
		return "", 0, err
	}

	port, err := strconv.ParseUint(portHex, 16, 16)
	if err != nil {
		return "", 0, fmt.Errorf("bad port: %v", err)
	}

	return ip.String(), uint16(port), nil
}

func parseIPv4Hex(hexStr string) (net.IP, error) {
	b, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, err
	}
	if len(b) != 4 {
		return nil, fmt.Errorf("ipv4 hex length mismatch")
	}

	// Little Endian
	return net.IPv4(b[3], b[2], b[1], b[0]), nil
}

func parseIPv6Hex(hexStr string) (net.IP, error) {
	b, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, err
	}
	if len(b) != 16 {
		return nil, fmt.Errorf("ipv6 hex length mismatch")
	}

	ip := make(net.IP, 16)
	for i := 0; i < 16; i += 4 {
		ip[i] = b[i+3]
		ip[i+1] = b[i+2]
		ip[i+2] = b[i+1]
		ip[i+3] = b[i]
	}

	return ip, nil
}
