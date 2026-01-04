//go:build windows

package diagnostics

import (
	"context"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func FetchLogs(ctx context.Context, opts protocol.LogRequest) ([]protocol.LogEntry, error) {
	return nil, nil
}
