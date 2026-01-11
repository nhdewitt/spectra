package agent

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// postCompressed marshals data to JSON, compresses it, and sends it to the server.
func postCompressed(ctx context.Context, client *http.Client, url string, data any) error {
	// Marshal JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshalling failed: %w", err)
	}

	// Compress
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	if _, err := gw.Write(jsonData); err != nil {
		return fmt.Errorf("compression failed: %w", err)
	}
	if err := gw.Close(); err != nil {
		return fmt.Errorf("gzip close failed: %w", err)
	}

	// Create Request
	req, err := http.NewRequestWithContext(ctx, "POST", url, &buf)
	if err != nil {
		return fmt.Errorf("request creation failed: %w", err)
	}

	// Headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")

	// Send
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("server rejected with status: %s", resp.Status)
	}

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
