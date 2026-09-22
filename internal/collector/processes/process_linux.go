//go:build linux

package processes

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"

	"github.com/nhdewitt/spectra/internal/collector/memory"
	"github.com/tklauser/go-sysconf"
)

var clkTck = 100.0

// statBuf size is comfortably larger than any /proc/<pid>/stat line. The buffer
// is reused across every PID in a pass rather than allocated per process.
const statBufSize = 2048

// lastStateField is the highest index we read out of the tail of the line, so
// splitStatFields can stop there instead of walking all fields.
const lastStatField = 21

func init() {
	if sc, err := sysconf.Sysconf(sysconf.SC_CLK_TCK); err == nil && sc > 0 {
		clkTck = float64(sc)
	}
}

// pidStatRaw holds the raw values parsed from /proc/[pid]/stat
type pidStatRaw struct {
	Name       string
	State      string
	PPID       int
	UTime      uint64
	STime      uint64
	RSSPages   uint64
	TotalTicks uint64
	NumThreads uint32
}

func getRAMTotal() uint64 {
	return memory.Total()
}

func collectRaw() ([]processRaw, int64, error) {
	totalMem := getRAMTotal()

	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, 0, err
	}

	pageSize := uint64(os.Getpagesize())
	procs := make([]processRaw, 0, len(entries))
	buf := make([]byte, statBufSize)

	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}

		n, err := readStat(filepath.Join("/proc", entry.Name(), "stat"), buf)
		if err != nil {
			continue
		}

		stat, err := parsePidStat(buf[:n])
		if err != nil {
			continue
		}

		procs = append(procs, processRaw{
			PID:        pid,
			Name:       stat.Name,
			State:      stat.State,
			RSSBytes:   stat.RSSPages * pageSize,
			TotalTicks: stat.TotalTicks,
			NumThreads: stat.NumThreads,
		})
	}

	return procs, int64(totalMem), nil
}

// readStat fills buf from path.
func readStat(path string, buf []byte) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	total := 0
	for total < len(buf) {
		n, err := f.Read(buf[total:])
		total += n
		if err == io.EOF {
			break
		}
		if err != nil {
			return total, err
		}
		if n == 0 {
			break
		}
	}
	return total, nil
}

// parsePidStatFrom parses a single line from /proc/<pid>/stat.
func parsePidStatFrom(r io.Reader) (*pidStatRaw, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return parsePidStat(data)
}

// parsePidStat parses the line in place. It holds no reference to data beyond
// the call except Name (which is copied). collectRaw reuses one buffer for
// every process in a pass.
func parsePidStat(data []byte) (*pidStatRaw, error) {
	firstParen := bytes.IndexByte(data, '(')
	lastParen := bytes.LastIndexByte(data, ')')
	if firstParen == -1 || lastParen == -1 || lastParen <= firstParen {
		return nil, errors.New("invalid format")
	}

	// A process name can contain spaces and parens, which is why the name is
	// bounded by the outermost pair rather than split on whitespace.
	name := string(data[firstParen+1 : lastParen])

	if lastParen+2 > len(data) {
		return nil, errors.New("insufficient fields")
	}

	var fields [lastStatField + 1][]byte
	if splitStatFields(data[lastParen+2:], fields[:]) <= lastStatField {
		return nil, errors.New("insufficient fields")
	}

	// Indices shifted:
	// State (2) -> 0, PPID (4) -> 1, utime (14) -> 11,
	// stime (15) -> 12, num_threads (20) -> 17, rss (24) -> 21
	ppid := int(parseField(fields[1], 1))
	utime := parseField(fields[11], 11)
	stime := parseField(fields[12], 12)
	numThreads := parseField(fields[17], 17)
	rss := parseField(fields[21], 21)

	return &pidStatRaw{
		Name:       name,
		State:      string(fields[0]),
		PPID:       ppid,
		UTime:      utime,
		STime:      stime,
		RSSPages:   rss,
		TotalTicks: utime + stime,
		NumThreads: uint32(numThreads),
	}, nil
}

// splitStatFields fills out with the first len(out) space-separated fields of
// rest and returns how many it found.
func splitStatFields(rest []byte, out [][]byte) int {
	n, i := 0, 0
	for n < len(out) {
		for i < len(rest) && rest[i] == ' ' {
			i++
		}
		if i >= len(rest) {
			break
		}
		start := i
		for i < len(rest) && rest[i] != ' ' {
			i++
		}
		out[n] = rest[start:i]
		n++
	}
	return n
}

// parseField reads an unsigned decimal field, logging and returning zero rather than
// failing the whole process.
func parseField(b []byte, index int) uint64 {
	v, err := strconv.ParseUint(string(b), 10, 64)
	if err != nil {
		slog.Warn("field parse failed", "source", "process", "index", index, "value", string(b), "error", err)
		return 0
	}
	return v
}
