//go:build darwin && !cgo

package memory

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// vmStatTimeout bounds the subprocess. vm_stat returns immediately in
// normal operation. This only exists so a wedged process can't stall
// the collector.
const vmStatTimeout = 5 * time.Second

// SwapPaging returns swap-in and swap-out rates in pages per second
// or (nil, nil) when unavailable.
//
// This parses vm_stat, which reports the same counters as cgo's
// host_statistics64.
func SwapPaging() (*float64, *float64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), vmStatTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, "vm_stat").Output()
	if err != nil {
		return nil, nil, fmt.Errorf("running vm_stat: %w", err)
	}

	raw, err := parseVmStatOutput(out)
	if err != nil {
		return nil, nil, err
	}

	inRate, outRate := swapRates(raw, time.Now())
	return inRate, outRate, nil
}

// parseVmStatOutput reads the Swapins/Swapouts counters from vm_stat output.
//
// Lines look like "Swapins:	12345." (the trailing period is part of vm_stat's
// output, not a typo).
func parseVmStatOutput(out []byte) (swapRaw, error) {
	var raw swapRaw
	var haveIn, haveOut bool

	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		label, value, ok := strings.Cut(scanner.Text(), ":")
		if !ok {
			continue
		}

		switch strings.TrimSpace(label) {
		case "Swapins":
			n, err := parseVmStatCount(value)
			if err != nil {
				return swapRaw{}, fmt.Errorf("vm_stat Swapins: %w", err)
			}
			raw.In, haveIn = n, true
		case "Swapouts":
			n, err := parseVmStatCount(value)
			if err != nil {
				return swapRaw{}, fmt.Errorf("vm_stat Swapouts: %w", err)
			}
			raw.Out, haveOut = n, true
		default:
			continue
		}

		if haveIn && haveOut {
			return raw, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return swapRaw{}, fmt.Errorf("reading vm_stat output: %w", err)
	}

	var missingCounter string
	if !haveIn {
		missingCounter = "Swapins"
	} else {
		missingCounter = "Swapouts"
	}
	return swapRaw{}, fmt.Errorf("vm_stat: missing %s counter", missingCounter)
}

func parseVmStatCount(value string) (uint64, error) {
	return strconv.ParseUint(strings.TrimSuffix(strings.TrimSpace(value), "."), 10, 64)
}
