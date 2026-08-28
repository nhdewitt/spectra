//go:build linux

package memory

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

// SwapPaging returns swap-in and swap-out rates in pages per second
// or (nil, nil) when unavailable.
//
// Source is /proc/vmstat (cumulative page counters).
func SwapPaging() (*float64, *float64, error) {
	raw, err := parseVmStat()
	if err != nil {
		return nil, nil, err
	}

	in, out := swapRates(raw, time.Now())
	return in, out, nil
}

func parseVmStat() (swapRaw, error) {
	f, err := os.Open("/proc/vmstat")
	if err != nil {
		return swapRaw{}, fmt.Errorf("opening /proc/vmstat: %w", err)
	}
	defer f.Close()

	return parseVmStatFrom(f)
}

func parseVmStatFrom(r io.Reader) (swapRaw, error) {
	var raw swapRaw
	var haveIn, haveOut bool

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		// Relevant lines are "key value" with a single space
		key, value, ok := strings.Cut(scanner.Text(), " ")
		if !ok {
			continue
		}

		switch key {
		case "pswpin", "pswpout":
		default:
			continue
		}

		n, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
		if err != nil {
			return swapRaw{}, fmt.Errorf("/proc/vmstat %s: %w", key, err)
		}

		if key == "pswpin" {
			raw.In, haveIn = n, true
		} else {
			raw.Out, haveOut = n, true
		}

		if haveIn && haveOut {
			return raw, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return swapRaw{}, fmt.Errorf("reading /proc/vnstat: %w", err)
	}

	var missingCounter string
	if !haveIn {
		missingCounter = "pswpin"
	} else {
		missingCounter = "pswpout"
	}

	return swapRaw{}, fmt.Errorf("/proc/vmstat: missing %s counter", missingCounter)
}
