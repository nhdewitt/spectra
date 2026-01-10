package diagnostics

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

const (
	icmpv4EchoReply       = 0
	icmpv4DestUnreachable = 3
	icmpv4EchoRequest     = 8
	icmpv4TimeExceeded    = 11
)

func runPing(ctx context.Context, target string) ([]protocol.PingResult, error) {
	// Resolve IP
	if target == "" {
		return nil, fmt.Errorf("target cannot be empty")
	}
	ipAddr, err := net.ResolveIPAddr("ip4", target)
	if err != nil {
		return nil, fmt.Errorf("dns resolve failed: %v", err)
	}

	// Open Raw Socket
	conn, err := net.DialIP("ip4:icmp", nil, ipAddr)
	if err != nil {
		return nil, fmt.Errorf("socket open failed: %v", err)
	}
	defer conn.Close()

	results := make([]protocol.PingResult, 0, 4)
	pid := uint16(os.Getpid() & 0xffff)
	payload := []byte("SPECTRA-PING")
	readBuf := make([]byte, 1500)

	for seq := range 4 {
		if ctx.Err() != nil {
			return results, ctx.Err()
		}

		result := protocol.PingResult{Seq: seq}

		reqBytes := marshalMsg(icmpv4EchoRequest, pid, uint16(seq), payload)
		start := time.Now()

		if _, err := conn.Write(reqBytes); err != nil {
			result.Response = fmt.Sprintf("write failed: %v", err)
			results = append(results, result)
			continue
		}

		deadline := time.Now().Add(2 * time.Second)
		if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
			deadline = ctxDeadline
		}
		conn.SetReadDeadline(deadline)

		for {
			if ctx.Err() != nil {
				return results, ctx.Err()
			}

			n, peer, err := conn.ReadFrom(readBuf)
			if err != nil {
				result.Response = "timeout"
				break
			}

			// Packet too small for IP header
			if n < 20 {
				continue
			}

			// Parse IHL: lower 4 bits of byte 0
			headerLen := int(readBuf[0]&0x0F) * 4
			if n < headerLen+8 {
				continue
			}

			// Extract ICMP
			icmpData := readBuf[headerLen:n]
			icmpType := icmpData[0]

			// Ownership check
			found := false
			switch icmpType {
			case icmpv4EchoReply:
				// [Type][Code][Chk]*[ID][Seq]*[Data]
				recID := binary.BigEndian.Uint16(icmpData[4:6])
				recSeq := binary.BigEndian.Uint16(icmpData[6:8])

				// Check pid match and seq match
				if recID == pid && recSeq == uint16(seq) {
					result.Success = true
					result.RTT = time.Since(start)
					result.Response = "reply"
					result.Peer = peer.String()
					found = true
				}

			case icmpv4DestUnreachable, icmpv4TimeExceeded:
				// [Type][Code][Chk][Unused][Original IP Header][Original ICMP Header]
				// Need len of at least 8 bytes (outer ICMP) + 20 bytes (min. inner IP) + 8 bytes (inner ICMP)
				if len(icmpData) < 8+20+8 {
					continue
				}

				// Skip outer header
				innerData := icmpData[8:]

				// Parse inner IP header
				innerIHL := int(innerData[0]&0x0F) * 4
				if len(innerData) < innerIHL+8 {
					continue
				}

				// Parse inner ICMP header
				innerICMP := innerData[innerIHL:]
				if innerICMP[0] == icmpv4EchoRequest {
					origID := binary.BigEndian.Uint16(innerICMP[4:6])
					origSeq := binary.BigEndian.Uint16(innerICMP[6:8])

					if origID == pid && origSeq == uint16(seq) {
						result.Code = icmpData[1]
						result.Peer = peer.String()
						if icmpType == icmpv4TimeExceeded {
							result.Response = "ttl exceeded"
						} else {
							result.Response = "dest unreachable"
						}
						found = true
					}
				}
			}

			if found {
				break
			}
		}

		results = append(results, result)

		if seq < 3 {
			time.Sleep(1 * time.Second)
		}
	}

	return results, nil
}

func marshalMsg(typ uint8, id, seq uint16, payload []byte) []byte {
	b := make([]byte, 8+len(payload))

	b[0] = typ
	b[1] = 0
	binary.BigEndian.PutUint16(b[4:6], id)
	binary.BigEndian.PutUint16(b[6:8], seq)
	copy(b[8:], payload)

	csum := calculateChecksum(b)
	binary.BigEndian.PutUint16(b[2:4], csum)
	return b
}

func calculateChecksum(data []byte) uint16 {
	sum := 0

	for i := 0; i < len(data)-1; i += 2 {
		sum += int(data[i])<<8 | int(data[i+1])
	}
	if len(data)%2 == 1 {
		sum += int(data[len(data)-1]) << 8
	}

	// Fold 32-bit to 16 bits
	sum = (sum >> 16) + (sum & 0xffff)
	sum += (sum >> 16)
	return uint16(^sum)
}
