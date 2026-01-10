package diagnostics

import (
	"encoding/binary"
	"testing"
)

func TestMarshalMsg(t *testing.T) {
	tests := []struct {
		name    string
		typ     uint8
		id      uint16
		seq     uint16
		payload []byte
	}{
		{
			name:    "echo request",
			typ:     icmpv4EchoRequest,
			id:      0x1234,
			seq:     0x0001,
			payload: []byte("SPECTRA-PING"),
		},
		{
			name:    "empty payload",
			typ:     icmpv4EchoRequest,
			id:      0xABCD,
			seq:     0x0000,
			payload: []byte{},
		},
		{
			name:    "max seq",
			typ:     icmpv4EchoRequest,
			id:      0xFFFF,
			seq:     0xFFFF,
			payload: []byte("X"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := marshalMsg(tt.typ, tt.id, tt.seq, tt.payload)

			// Verify length
			expectedLen := 8 + len(tt.payload)
			if len(got) != expectedLen {
				t.Errorf("length: got %d, want %d", len(got), expectedLen)
			}

			// Verify type
			if got[0] != tt.typ {
				t.Errorf("type: got %d, want %d", got[0], tt.typ)
			}

			// Verify code is 0
			if got[1] != 0 {
				t.Errorf("code: got %d, want 0", got[1])
			}

			// Verify ID
			gotID := binary.BigEndian.Uint16(got[4:6])
			if gotID != tt.id {
				t.Errorf("id: got %04x, want %04x", gotID, tt.id)
			}

			// Verify seq
			gotSeq := binary.BigEndian.Uint16(got[6:8])
			if gotSeq != tt.seq {
				t.Errorf("seq: got %04x, want %04x", gotSeq, tt.seq)
			}

			// Verify payload
			gotPayload := got[8:]
			if string(gotPayload) != string(tt.payload) {
				t.Errorf("payload: got %q, want %q", gotPayload, tt.payload)
			}

			// Verify checksum is valid
			gotChecksum := binary.BigEndian.Uint16(got[2:4])
			got[2], got[3] = 0, 0
			expectedChecksum := calculateChecksum(got)
			if gotChecksum != expectedChecksum {
				t.Errorf("checksum: got %04x, want %04x", gotChecksum, expectedChecksum)
			}
		})
	}
}

func TestCalculateChecksum(t *testing.T) {
	t.Run("all zeros gives 0xFFFF", func(t *testing.T) {
		got := calculateChecksum([]byte{0x00, 0x00, 0x00, 0x00})
		if got != 0xFFFF {
			t.Errorf("got %04x, want 0xFFFF", got)
		}
	})

	t.Run("empty gives 0xFFFF", func(t *testing.T) {
		got := calculateChecksum([]byte{})
		if got != 0xFFFF {
			t.Errorf("got %04x, want 0xFFFF", got)
		}
	})

	t.Run("checksum is deterministic", func(t *testing.T) {
		data := []byte{0x08, 0x00, 0x00, 0x00, 0x12, 0x34, 0x00, 0x01}
		first := calculateChecksum(data)
		second := calculateChecksum(data)
		if first != second {
			t.Errorf("non-deterministic: %04x != %04x", first, second)
		}
	})
}

func TestChecksumRoundTrip(t *testing.T) {
	payload := []byte("SPECTRA-PING")
	msg := marshalMsg(icmpv4EchoRequest, 0x1234, 0x0001, payload)

	verify := calculateChecksum(msg)
	if verify != 0x0000 {
		t.Errorf("round-trip failed: got %04x, want 0x0000", verify)
	}
}
