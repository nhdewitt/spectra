package diagnostics

import (
	"context"
	"fmt"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func RunNetworkDiag(ctx context.Context, req protocol.NetworkRequest) (*protocol.NetworkDiagnosticReport, error) {
	report := &protocol.NetworkDiagnosticReport{
		Action: req.Action,
		Target: req.Target,
	}

	var err error
	switch req.Action {
	case "ping":
		results, err := runPing(ctx, req.Target)
		report.PingResults = results
		if err != nil {
			return report, err
		}

	case "traceroute":
		report.RawOutput, err = runTraceroute(ctx, req.Target)

	case "netstat":
		report.Target = "Local System"
		report.Netstat, err = getNetstat(ctx)

	case "connect":
		res := testConnectivity(req.Target, 3*time.Second)
		report.PingResults = []protocol.PingResult{res}
		return report, nil

	default:
		return nil, fmt.Errorf("unknown network action: %s", req.Action)
	}

	return report, err
}
