//go:build freebsd

package collector

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
	"golang.org/x/sys/unix"
)

func CollectSystem(ctx context.Context) ([]protocol.Metric, error) {
	tv, err := unix.SysctlTimeval("kern.boottime")
	if err != nil {
		return nil, err
	}
	bootTime := uint64(tv.Sec)
	uptime := uint64(time.Now().Unix()) - bootTime

	procCount, _ := countProcs() // fallback to 0 if we can't count the processes

	out, _ := exec.CommandContext(ctx, "who").Output()
	users := parseWhoFrom(bytes.NewReader(out))

	return []protocol.Metric{
		protocol.SystemMetric{
			Uptime:    uptime,
			BootTime:  bootTime,
			Processes: procCount,
			Users:     users,
		},
	}, nil
}

// parseWhoFrom counts lines in the output of the `who` command.
func parseWhoFrom(r io.Reader) int {
	data, err := io.ReadAll(r)
	if err != nil {
		return 0
	}

	s := strings.TrimSpace(string(data))
	if s == "" {
		return 0
	}

	return len(strings.Split(s, "\n"))
}

func countProcs() (int, error) {
	buf, err := unix.SysctlRaw("kern.proc.proc", 0)
	if err != nil {
		return 0, err
	}
	if len(buf) < 4 {
		return 0, err
	}

	var structSize int32
	binary.Read(bytes.NewReader(buf[:4]), binary.LittleEndian, &structSize)
	if structSize <= 0 {
		return 0, nil
	}

	return len(buf) / int(structSize), nil
}
