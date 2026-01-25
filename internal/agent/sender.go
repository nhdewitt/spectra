package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

const (
	BatchSize    = 100             // If we reach this, send immediately
	SendInterval = 5 * time.Second // Force sending every 5 seconds
)

// runMetricSender consumes the channel and sends batches via HTTP
func (a *Agent) runMetricSender() {
	batch := make([]protocol.Envelope, 0, BatchSize)

	ticker := time.NewTicker(SendInterval)
	defer ticker.Stop()

	flush := func() {
		if len(batch) > 0 {
			a.uploadBatch(batch)
			batch = batch[:0]
		}
	}

	for {
		select {
		case envelope, ok := <-a.metricsCh:
			if !ok {
				flush()
				return
			}
			batch = append(batch, envelope)
			if len(batch) >= BatchSize {
				flush()
			}

		case <-ticker.C:
			flush()

		case <-a.ctx.Done():
			flush()
			return
		}
	}
}

func (a *Agent) uploadBatch(batch []protocol.Envelope) {
	url := fmt.Sprintf("%s%s?hostname=%s", a.Config.BaseURL, a.Config.MetricsPath, a.Config.Hostname)

	if err := a.postCompressed(url, batch); err != nil {
		fmt.Printf("Error sending batch of %d metrics: %v\n", len(batch), err)
	} else {
		fmt.Printf("Sent batch of %d metrics\n", len(batch))
	}
}

// postCompressed marshals data to JSON, compresses it, and sends it to the server.
func (a *Agent) postCompressed(url string, batch []protocol.Envelope) error {
	a.gzipMu.Lock()
	a.gzipBuf.Reset()
	a.gzipW.Reset(&a.gzipBuf)

	if err := json.NewEncoder(a.gzipW).Encode(batch); err != nil {
		return fmt.Errorf("json encode error: %w", err)
	}

	if err := a.gzipW.Close(); err != nil {
		return fmt.Errorf("gzip close error: %w", err)
	}

	payload := append([]byte(nil), a.gzipBuf.Bytes()...)
	a.gzipMu.Unlock()

	req, err := http.NewRequestWithContext(a.ctx, "POST", url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create request error: %w", err)
	}

	a.setHeaders(req)

	resp, err := a.Client.Do(req)
	if err != nil {
		return fmt.Errorf("http error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	return nil
}
