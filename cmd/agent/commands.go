package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/nhdewitt/spectra/internal/collector"
	"github.com/nhdewitt/spectra/internal/diagnostics"
	"github.com/nhdewitt/spectra/internal/protocol"
)

// runCommandLoop long-polls the server for tasks
func runCommandLoop(ctx context.Context, client *http.Client, cfg Config, driveCache *collector.DriveCache) {
	url := fmt.Sprintf("%s%s?hostname=%s", cfg.BaseURL, cfg.CommandPath, cfg.Hostname)
	fmt.Println("Starting Command & Control loop at", url)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
			if err != nil {
				fmt.Printf("Error creating request: %v\n", err)
				continue
			}

			resp, err := client.Do(req)
			if err != nil {
				fmt.Printf("C2 Connection failed: %v\n", err)
				continue
			}

			switch resp.StatusCode {
			case http.StatusOK:
				var cmd protocol.Command
				if err := json.NewDecoder(resp.Body).Decode(&cmd); err == nil {
					go handleCommand(ctx, client, cfg, cmd, driveCache)
				}
			case http.StatusNoContent:
			default:
				time.Sleep(10 * time.Second)
			}
			resp.Body.Close()
		}
	}
}

func handleCommand(ctx context.Context, client *http.Client, cfg Config, cmd protocol.Command, driveCache *collector.DriveCache) {
	fmt.Printf("Received Command: %s (%s)\n", cmd.Type, cmd.ID)

	var resultData any
	var err error

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
				targetPath = driveCache.GetDefaultPath()
			}

			if req.TopN == 0 {
				req.TopN = 50
			}

			resultData, err = diagnostics.RunDiskUsageTop(ctx, targetPath, req.TopN, req.TopN)
		}

	case protocol.CmdRestartAgent:
		err = fmt.Errorf("restart not implemented yet")

	case protocol.CmdListMounts:
		resultData = driveCache.ListMounts()

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

	if uploadErr := uploadCommandResult(ctx, client, cfg, cmd, resultData, err); uploadErr != nil {
		fmt.Printf("Failed to upload result for %s: %v\n", cmd.ID, uploadErr)
	}
}

// uploadCommandResult handles JSON marshaling, Gzip compression, and HTTP transport.
func uploadCommandResult(ctx context.Context, client *http.Client, cfg Config, cmd protocol.Command, data interface{}, cmdErr error) error {
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

	// Marshal the envelope
	envelopeBytes, err := json.Marshal(res)
	if err != nil {
		return err
	}

	// Compress
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	if _, err := gw.Write(envelopeBytes); err != nil {
		return err
	}
	if err := gw.Close(); err != nil {
		return err
	}
	compressedSize := buf.Len()

	// Send
	url := fmt.Sprintf("%s/api/v1/agent/command_result?hostname=%s", cfg.BaseURL, cfg.Hostname)

	req, err := http.NewRequestWithContext(ctx, "POST", url, &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")

	resp, err := client.Do(req)
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

func formatBytes(b int) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
