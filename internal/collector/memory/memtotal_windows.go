//go:build windows

package memory

import (
	"fmt"
	"unsafe"

	"github.com/nhdewitt/spectra/internal/winapi"
)

// totalPhysical reads ullTotalPhys from GlobalMemoryStatusEx. Collect makes the
// same call for its own reasons, this is separate so the value is available at
// agent startup, before any collector has run.
func totalPhysical() (uint64, error) {
	var memStatus winapi.MemoryStatusEx
	memStatus.Length = uint32(unsafe.Sizeof(memStatus))

	ret, _, _ := winapi.ProcGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&memStatus)))
	if ret == 0 {
		return 0, fmt.Errorf("GlobalMemoryStatusEx failed")
	}

	return memStatus.TotalPhys, nil
}
