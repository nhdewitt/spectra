//go:build darwin

package wifi

import (
	"context"

	"github.com/nhdewitt/spectra/internal/protocol"
)

// Collect is a no-op on Darwin. Apple removed the airport utility in 14.4,
// disabled networksetup -getairportnetwork in 15, and wdutil requires root.
// system_profiler SPAirPortDataType still reports signal and channel, but
// takes seconds per call and returns no SSID: since macOS 15 that requires
// Location Services permission, granted only to Developer ID-signed apps
// that can prompt a user. A launchd daemon cannot hold that grant, and cgo
// does not change it. Returning nil with a nil error is correct -- nothing
// failed, there is nothing to report, and Run treats an empty slice as
// success.
func Collect(ctx context.Context) ([]protocol.Metric, error) {
	return nil, nil
}
