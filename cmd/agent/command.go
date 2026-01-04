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

	"github.com/nhdewitt/spectra/internal/diagnostics"
	"github.com/nhdewitt/spectra/internal/protocol"
)

// runCommandLoop long-polls the server for tasks
func runCommandLoop(ctx context.Context, cfg Config) {
	client := &http.Client{
		Timeout: 40 * time.Second,
	}

	url := fmt.Sprintf("%s%s?hostname=%s", cfg.BaseURL, cfg.CommandPath, cfg.Hostname)
	fmt.Println("Starting Command & Control loop at", url)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
			if err != nil {
				time.Sleep(5 * time.Second)
				continue
			}

			resp, err := client.Do(req)
			if err != nil {
				time.Sleep(10 * time.Second)
				continue
			}

			switch resp.StatusCode {
			case http.StatusOK:
				var cmd protocol.Command
				if err := json.NewDecoder(resp.Body).Decode(&cmd); err == nil {
					go handleCommand(ctx, client, cfg, cmd)
				}
			case http.StatusNoContent:
			default:
				time.Sleep(10 * time.Second)
			}
			resp.Body.Close()
		}
	}
}

func handleCommand(ctx context.Context, client *http.Client, cfg Config, cmd protocol.Command) {
	fmt.Printf("Received Command: %s (%s)\n", cmd.Type, cmd.ID)

	switch cmd.Type {
	case protocol.CmdFetchLogs:
		var req protocol.LogRequest
		if err := json.Unmarshal(cmd.Payload, &req); err != nil {
			fmt.Println("Error parsing log request:", err)
			return
		}

		logs, err := diagnostics.FetchLogs(ctx, req)
		if err != nil {
			fmt.Println("Error fetching logs:", err)
			return
		}

		uploadLogs(ctx, client, cfg, logs, cmd.ID)

	case protocol.CmdRestartAgent:
		fmt.Println("Restart requested (not implemented)")
	}
}

func uploadLogs(ctx context.Context, client *http.Client, cfg Config, logs []protocol.LogEntry, cmdID string) {
	fmt.Printf("uploadLogs: preparing %d logs for cmd_id=%s\n", len(logs), cmdID)

	data, err := json.Marshal(logs)
	if err != nil {
		return
	}

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	if _, err := gw.Write(data); err != nil {
		fmt.Println("uploadLogs: gzip write:", err)
		_ = gw.Close()
		return
	}
	if err := gw.Close(); err != nil {
		fmt.Println("uploadLogs: gzip close:", err)
		return
	}

	url := fmt.Sprintf("%s%s?cmd_id=%s&hostname=%s", cfg.BaseURL, cfg.LogsPath, cmdID, cfg.Hostname)
	fmt.Println("uploadLogs: POST", url, "bytes=", buf.Len())

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(buf.Bytes()))
	if err != nil {
		fmt.Println("uploadLogs: new request:", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("uploadLogs: do:", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("uploadLogs: resp %s\n%s\n", resp.Status, string(body))
	if resp.StatusCode/100 != 2 {
		fmt.Printf("Upload failed: %s\n%s\n", resp.Status, string(body))
		return
	}

	fmt.Printf("Uploaded %d log entries (%s compressed).\n", len(logs), formatBytes(buf.Len()))
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
