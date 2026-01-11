package agent

import (
	"fmt"
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
			batch = make([]protocol.Envelope, 0, BatchSize)
		}
	}

	for {
		select {
		case envelope := <-a.metricsCh:
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

	if err := postCompressed(a.ctx, a.Client, url, batch); err != nil {
		fmt.Printf("Error sending batch of %d metrics: %v\n", len(batch), err)
	} else {
		fmt.Printf("Sent batch of %d metrics\n", len(batch))
	}
}
