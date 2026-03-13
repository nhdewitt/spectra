//go:build darwin

package cpu

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"golang.org/x/sys/unix"
)

type darwinLoadAvg struct {
	Ldavg  [3]uint32
	_      [4]byte // padding
	Fscale int64
}

// parseLoadAvg reads load averages from sysctl vm.loadavg.
//
// Darwin struct:
//
//	struct loadavg {
//		fixpt_t ldavg[3];
//		long fscale;
//	}
//
// 12 bytes (ldavg) + 4 padding + 8 bytes (fscale) = 24 bytes
func parseLoadAvg() (load1, load5, load15 float64, err error) {
	buf, err := unix.SysctlRaw("vm.loadavg")
	if err != nil {
		return 0, 0, 0, err
	}

	return parseLoadAvgBuf(buf)
}

func parseLoadAvgBuf(buf []byte) (load1, load5, load15 float64, err error) {
	var raw darwinLoadAvg

	if len(buf) < 24 {
		return 0, 0, 0, fmt.Errorf("vm.loadavg: expected %d bytes, got %d", binary.Size(raw), len(buf))
	}

	if err := binary.Read(bytes.NewReader(buf), binary.LittleEndian, &raw); err != nil {
		return 0, 0, 0, fmt.Errorf("vm.loadavg: %w", err)
	}
	if raw.Fscale == 0 {
		return 0, 0, 0, fmt.Errorf("fscale is zero")
	}

	fscale := float64(raw.Fscale)
	load1 = float64(raw.Ldavg[0]) / fscale
	load5 = float64(raw.Ldavg[1]) / fscale
	load15 = float64(raw.Ldavg[2]) / fscale

	return load1, load5, load15, nil
}
