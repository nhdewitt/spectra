//go:build darwin

package collector

import (
	"encoding/binary"
	"testing"
)

func TestParseLoadAvg_Integration(t *testing.T) {
	l1, l5, l15, err := parseLoadAvg()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if l1 < 0 || l5 < 0 || l15 < 0 {
		t.Errorf("negative load averages: %.2f %.2f %.2f", l1, l5, l15)
	}

	if l1 == 0 && l5 == 0 && l15 == 0 {
		t.Error("all load averages are zero")
	}

	t.Logf("load averages: %.2f %.2f %.2f", l1, l5, l15)
}

func TestParseLoadAvgBuf_TooShort(t *testing.T) {
	_, _, _, err := parseLoadAvgBuf(make([]byte, 10))
	if err == nil {
		t.Error("expected error for short buffer")
	}
}

func TestParseLoadAvgBuf_ZeroFscale(t *testing.T) {
	buf := make([]byte, binary.Size(darwinLoadAvg{}))
	binary.LittleEndian.PutUint32(buf[0:4], 100)

	_, _, _, err := parseLoadAvgBuf(buf)
	if err == nil {
		t.Error("expected error for zero fscale")
	}
}
