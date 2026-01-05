package main

import (
	"context"
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
func runMetricSender(ctx context.Context, client *http.Client, cfg Config, ch <-chan protocol.Envelope) {
	batch := make([]protocol.Envelope, 0, BatchSize)

	ticker := time.NewTicker(SendInterval)
	defer ticker.Stop()

	flush := func() {
		if len(batch) > 0 {
			uploadBatch(ctx, client, cfg, batch)
			batch = make([]protocol.Envelope, 0, BatchSize)
		}
	}

	for {
		select {
		case envelope := <-ch:
			batch = append(batch, envelope)
			if len(batch) >= BatchSize {
				flush()
			}

		case <-ticker.C:
			flush()

		case <-ctx.Done():
			flush()
			return
		}
	}
}

func uploadBatch(ctx context.Context, client *http.Client, cfg Config, batch []protocol.Envelope) {
	url := fmt.Sprintf("%s%s?hostname=%s", cfg.BaseURL, cfg.MetricsPath, cfg.Hostname)

	if err := postCompressed(ctx, client, url, batch); err != nil {
		fmt.Printf("Error sending batch of %d metrics: %v\n", len(batch), err)
	} else {
		fmt.Printf("Sent batch of %d metrics\n", len(batch))
	}
}
