//go:build freebsd

package memory

import (
	"fmt"
	"time"

	"golang.org/x/sys/unix"
)

// SwapPaging returns swap-in and swap-out rates in pages per second
// or (nil, nil) when unavailable.
//
// v_swappgsin/v_swappgsout are analogs of Linux's pswpin/pswpout.
func SwapPaging() (*float64, *float64, error) {
	in, err := unix.SysctlUint32("vm.stats.vm.v_swappgsin")
	if err != nil {
		return nil, nil, fmt.Errorf("sysctl v_swappgsin: %w", err)
	}

	out, err := unix.SysctlUint32("vm.stats.vm.v_swappgsout")
	if err != nil {
		return nil, nil, fmt.Errorf("sysctl v_swappgsout: %w", err)
	}

	inRate, outRate := swapRates(swapRaw{In: uint64(in), Out: uint64(out)}, time.Now())
	return inRate, outRate, nil
}
