//go:build linux

package memory

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

func parseMemInfo() (memRaw, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return memRaw{}, fmt.Errorf("opening /proc/meminfo: %w", err)
	}
	defer f.Close()

	return parseMemInfoFrom(f)
}

func parseMemInfoFrom(r io.Reader) (memRaw, error) {
	var raw memRaw

	targets := map[string]*uint64{
		"MemTotal":     &raw.Total,
		"MemAvailable": &raw.Available,
		"SwapTotal":    &raw.SwapTotal,
		"SwapFree":     &raw.SwapFree,
	}

	var commitLimit, commitUsed uint64
	optional := map[string]*uint64{
		"CommitLimit":  &commitLimit,
		"Committed_AS": &commitUsed,
	}
	found := make(map[string]bool, len(optional))

	scanner := bufio.NewScanner(r)

	for scanner.Scan() && len(targets)+len(optional) > 0 {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}

		key := strings.TrimSuffix(fields[0], ":")

		target, ok := targets[key]
		optionalTarget, optionalOK := optional[key]
		if !ok && !optionalOK {
			continue
		}
		if optionalOK {
			target = optionalTarget
		}

		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			if optionalOK {
				// An unparseable optional field is never fatal.
				delete(optional, key)
				continue
			}
			return memRaw{}, fmt.Errorf("parsing %s: %w", key, err)
		}

		*target = value * 1024
		// Remove the key to prevent duplicates from changing the value
		if ok {
			delete(targets, key)
		} else {
			delete(optional, key)
			found[key] = true
		}
	}

	if err := scanner.Err(); err != nil {
		return memRaw{}, fmt.Errorf("reading /proc/meminfo: %w", err)
	}
	if len(targets) > 0 {
		missing := make([]string, 0, len(targets))
		for k := range targets {
			missing = append(missing, k)
		}
		return memRaw{}, fmt.Errorf("missing fields in /proc/meminfo: %v", missing)
	}

	// Both or neither
	if found["CommitLimit"] && found["Committed_AS"] {
		raw.CommitLimit = &commitLimit
		raw.CommitUsed = &commitUsed
	}

	return raw, nil
}
