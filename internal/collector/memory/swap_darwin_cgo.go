//go:build darwin && cgo

package memory

/*
#include <mach/mach.h>

// Wraps host_statistics64 so the Go side never touches the mach_host_self
// macro directly.
static kern_return_t spectra_vm_stats(vm_statistics64_data_t *out) {
	mach_msg_type_number_t count = HOST_VM_INFO64_COUNT;
	mach_port_t host = mach_host_self();

	kern_return_t ret = host_statistics64(host, HOST_VM_INFO64, (host_info64_t)out, &count);

	mach_port_deallocate(mach_task_self(), host);
	return ret;
}
*/
import "C"

import (
	"fmt"
	"time"
)

// SwapPaging returns swap-in and swap-out rates in pages per second
// or (nil, nil) when unavailable.
func SwapPaging() (*float64, *float64, error) {
	var vmstat C.vm_statistics64_data_t

	if ret := C.spectra_vm_stats(&vmstat); ret != C.KERN_SUCCESS {
		return nil, nil, fmt.Errorf("host_statistics64: kern_return %d", int(ret))
	}

	in, out := swapRates(swapRaw{
		In:  uint64(vmstat.swapins),
		Out: uint64(vmstat.swapouts),
	}, time.Now())
	return in, out, nil
}
