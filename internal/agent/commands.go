package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/nhdewitt/spectra/internal/diagnostics"
	"github.com/nhdewitt/spectra/internal/protocol"
)

const (
	// commandExecTimeout bounds a single diagnostic run.
	commandExecTimeout = 60 * time.Second
	// updateExecTimeout bounds a self-update. A measured update on the slowest
	// hardware in the fleet took 4.5s to download and verify ~8MB, so this is
	// not an estimate of the work, it is a ceiling on how long a stuck one may
	// hang before being called dead. Sizing it near the observed time would
	// defeat that, since the case it guards is a half-open connection or a link
	// that has collapsed to a trickle, which by definition does not resemble
	// the measurement. Ten minutes covers ~15KB/s for a binary a third larger
	// than today's.
	updateExecTimeout = 10 * time.Minute
	// commandReportTimeout bounds uploading the result of one.
	commandReportTimeout = 30 * time.Second
)

// execTimeoutFor returns how long a command may run. Only the self-update
// differs; everything else is a diagnostic that should not hang around.
func execTimeoutFor(cmdType protocol.CommandType) time.Duration {
	switch cmdType {
	case protocol.CmdUpdateAgent:
		return updateExecTimeout
	default:
		return commandExecTimeout
	}
}

// runCommandLoop long-polls the server for tasks
func (a *Agent) runCommandLoop(ctx context.Context) {
	url := fmt.Sprintf("%s%s", a.Config.BaseURL, a.Config.CommandPath)
	a.Logger.Info("command loop started", "url", url)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.pollOnce(ctx, url)
		}
	}
}

func (a *Agent) pollOnce(ctx context.Context, url string) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		a.Logger.Error("failed to create command request", "error", err)
		return
	}
	a.setHeaders(req)
	req.Header.Del("Content-Encoding")

	resp, err := a.Client.Do(req)
	if err != nil {
		a.Logger.Debug("command poll failed", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var cmd protocol.Command
		if err := json.NewDecoder(resp.Body).Decode(&cmd); err == nil {
			go a.handleCommand(ctx, cmd)
		}
	}
}

// handleCommand runs one server-issued command and reports the outcome.
//
// Execution and reporting run on separate contexts. Reusing the execution
// context for the upload meant a diagnostic that hit its deadline tried to
// report "context deadline exceeded" over a context that was already canceled,
// so the single most useful result was the one that never arrived. The report
// context still derives from the caller's, so agent shutdown ends it.
//
// A successful update exits the process here, after the result is uploaded,
// rather than from a goroutine watchinga context inside selfUpdate.
func (a *Agent) handleCommand(ctx context.Context, cmd protocol.Command) {
	a.Logger.Info("command received", "type", cmd.Type, "id", cmd.ID)

	var resultData any
	var err error

	// bounds the diagnostic itself
	execCtx, cancelExec := context.WithTimeout(ctx, execTimeoutFor(cmd.Type))
	defer cancelExec()

	switch cmd.Type {
	case protocol.CmdFetchLogs:
		var req protocol.LogRequest
		if json.Unmarshal(cmd.Payload, &req) == nil {
			resultData, err = diagnostics.FetchLogs(execCtx, req)
		} else {
			err = fmt.Errorf("invalid log request payload")
		}

	case protocol.CmdDiskUsage:
		var req protocol.DiskUsageRequest
		if len(cmd.Payload) > 0 {
			if json.Unmarshal(cmd.Payload, &req) != nil {
				err = fmt.Errorf("invalid disk usage request payload")
			}
		}

		if err == nil {
			targetPath := req.Path
			if targetPath == "" {
				// Find the main drive (most likely "/" on Linux or "C:" on Windows)
				targetPath = a.DriveCache.GetDefaultPath()
			}

			if req.TopN == 0 {
				req.TopN = 50
			}

			resultData, err = diagnostics.RunDiskUsageTop(execCtx, targetPath, req.TopN, req.TopN)
		}

	case protocol.CmdRestartAgent:
		err = fmt.Errorf("restart not implemented yet")

	case protocol.CmdListMounts:
		resultData = a.DriveCache.ListMounts()

	case protocol.CmdNetworkDiag:
		var req protocol.NetworkRequest
		if json.Unmarshal(cmd.Payload, &req) == nil {
			resultData, err = diagnostics.RunNetworkDiag(execCtx, req)
		} else {
			err = fmt.Errorf("invalid network request payload")
		}

	case protocol.CmdUpdateAgent:
		var req protocol.UpdateAgentRequest
		if json.Unmarshal(cmd.Payload, &req) == nil {
			resultData, err = a.selfUpdate(execCtx, req)
		} else {
			err = fmt.Errorf("invalid update request payload")
		}

	default:
		err = fmt.Errorf("unknown command type: %s", cmd.Type)
	}

	// survives an execution timeout
	reportCtx, cancelReport := context.WithTimeout(ctx, commandReportTimeout)
	defer cancelReport()

	if uploadErr := a.uploadCommandResult(reportCtx, cmd, resultData, err); uploadErr != nil {
		a.Logger.Error("failed to upload command result", "command_id", cmd.ID, "error", uploadErr)
	}

	if restartRequested(cmd, resultData, err) {
		a.Logger.Info("exiting for service manager restart", "command_id", cmd.ID)
		os.Exit(0)
	}
}

// restartRequested reports whether a completed command installed a new binary
// and needs the process to exit so the service manager restarts it.
//
// The status check has to be exact. selfUpdate also returns successfully with
// UpdateStatusAlreadyCurrent, having installed nothing, and exiting on that
// would restart the agent every time the server pushed an update it already had.
func restartRequested(cmd protocol.Command, resultData any, err error) bool {
	if cmd.Type != protocol.CmdUpdateAgent || err != nil {
		return false
	}
	res, ok := resultData.(*protocol.UpdateAgentResult)
	return ok && res != nil && res.Status == protocol.UpdateStatusRestarting
}

// uploadCommandResult handles JSON marshaling, Gzip compression, and HTTP transport.
func (a *Agent) uploadCommandResult(ctx context.Context, cmd protocol.Command, data any, cmdErr error) error {
	res := protocol.CommandResult{
		ID:   cmd.ID,
		Type: cmd.Type,
	}

	if cmdErr != nil {
		res.Error = cmdErr.Error()
	} else if data != nil {
		raw, err := json.Marshal(data)
		if err != nil {
			res.Error = fmt.Sprintf("failed to marshal payload: %v", err)
		} else {
			res.Payload = raw
		}
	}

	payload, err := a.compressPayload(res)
	if err != nil {
		return fmt.Errorf("compression failed: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/agent/command/result", a.Config.BaseURL)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payload))
	if err != nil {
		return err
	}

	a.setHeaders(req)

	resp, err := a.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server rejected result (%s): %s", resp.Status, string(body))
	}

	a.Logger.Debug("command result uploaded", "command_id", cmd.ID, "compressed_bytes", len(payload))
	return nil
}

func (a *Agent) runNightly(ctx context.Context, hour, minute int, fn func()) {
	for {
		now := time.Now()
		next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())

		if now.After(next) {
			next = next.Add(24 * time.Hour)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Until(next)):
			fn()
		}
	}
}
