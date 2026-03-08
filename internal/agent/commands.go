package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/nhdewitt/spectra/internal/diagnostics"
	"github.com/nhdewitt/spectra/internal/protocol"
)

// runCommandLoop long-polls the server for tasks
func (a *Agent) runCommandLoop(ctx context.Context) {
	url := fmt.Sprintf("%s%s", a.Config.BaseURL, a.Config.CommandPath)
	fmt.Println("Starting Command & Control loop at", url)

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
		fmt.Printf("Error creating request: %v\n", err)
		return
	}
	a.setHeaders(req)
	req.Header.Del("Content-Encoding")

	resp, err := a.Client.Do(req)
	if err != nil {
		fmt.Printf("C2 connection failed: %v\n", err)
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

func (a *Agent) handleCommand(ctx context.Context, cmd protocol.Command) {
	fmt.Printf("Received Command: %s (%s)\n", cmd.Type, cmd.ID)

	var resultData any
	var err error

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	switch cmd.Type {
	case protocol.CmdFetchLogs:
		var req protocol.LogRequest
		if json.Unmarshal(cmd.Payload, &req) == nil {
			resultData, err = diagnostics.FetchLogs(ctx, req)
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

			resultData, err = diagnostics.RunDiskUsageTop(ctx, targetPath, req.TopN, req.TopN)
		}

	case protocol.CmdRestartAgent:
		err = fmt.Errorf("restart not implemented yet")

	case protocol.CmdListMounts:
		resultData = a.DriveCache.ListMounts()

	case protocol.CmdNetworkDiag:
		var req protocol.NetworkRequest
		if json.Unmarshal(cmd.Payload, &req) == nil {
			resultData, err = diagnostics.RunNetworkDiag(ctx, req)
		} else {
			err = fmt.Errorf("invalid network request payload")
		}

	default:
		err = fmt.Errorf("unknown command type: %s", cmd.Type)
	}

	if uploadErr := a.uploadCommandResult(ctx, cmd, resultData, err); uploadErr != nil {
		fmt.Printf("Failed to upload result for %s: %v\n", cmd.ID, uploadErr)
	}
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

	var payload []byte
	var compressedSize int

	err := func() error {
		a.gzipMu.Lock()
		defer a.gzipMu.Unlock()

		a.gzipBuf.Reset()
		a.gzipW.Reset(&a.gzipBuf)

		if err := json.NewEncoder(a.gzipW).Encode(res); err != nil {
			return err
		}
		if err := a.gzipW.Close(); err != nil {
			return err
		}

		compressedSize = a.gzipBuf.Len()
		payload = make([]byte, compressedSize)
		copy(payload, a.gzipBuf.Bytes())
		return nil
	}()
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

	fmt.Printf("Uploaded result for %s (%s compressed)\n", cmd.ID, formatBytes(compressedSize))
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
