//go:build freebsd

package collector

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

func parseMemInfo() (memRaw, error) {
	var raw memRaw

	physmem, err := unix.SysctlUint64("hw.physmem")
	if err != nil {
		return raw, fmt.Errorf("sysctl hw.physmem: %w", err)
	}
	raw.Total = physmem

	pageSize, err := unix.SysctlUint32("vm.stats.vm.v_page_size")
	if err != nil {
		return raw, fmt.Errorf("sysctl vm.stats.vm.v_page_size: %w", err)
	}
	ps := uint64(pageSize)

	// Available: (free + inactive) * pagesize
	// Matches what top reports as reclaimable.
	freeCount, err := unix.SysctlUint32("vm.stats.vm.v_free_count")
	if err != nil {
		return raw, fmt.Errorf("sysctl vm.stats.vm.v_inactive_count: %w", err)
	}
	inactiveCount, err := unix.SysctlUint32("vm.stats.vm.v_inactive_count")
	if err != nil {
		return raw, fmt.Errorf("sysctl vm.stats.vm.v_inactive_count: %w", err)
	}
	raw.Available = (uint64(freeCount) + uint64(inactiveCount)) * ps

	// Swap: parse swapinfo output
	swapTotal, swapUsed, err := getSwapInfo()
	if err != nil {
		// Not fatal - possibly no swap configured
		return raw, nil
	}
	raw.SwapTotal = swapTotal
	if swapTotal >= swapUsed {
		raw.SwapFree = swapTotal - swapUsed
	}

	return raw, nil
}

// getSwapInfo parses swapinfo -k and sums total/used swap across all devices.
func getSwapInfo() (total, used uint64, err error) {
	out, err := exec.Command("swapinfo", "-k").Output()
	if err != nil {
		return 0, 0, fmt.Errorf("swapinfo: %w", err)
	}

	scanner := bufio.NewScanner(bytes.NewReader(out))
	if !scanner.Scan() {
		return 0, 0, fmt.Errorf("swapinfo: empty output")
	}

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}

		t, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		u, err := strconv.ParseUint(fields[2], 10, 64)
		if err != nil {
			continue
		}

		total += t * 1024
		used += u * 1024
	}

	return total, used, scanner.Err()
}
