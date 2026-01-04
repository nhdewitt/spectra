package collector

import (
	"context"
	"log"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

type CollectFunc func(context.Context) ([]protocol.Metric, error)

type Collector struct {
	hostname string
	out      chan<- protocol.Envelope
}

func New(hostname string, out chan<- protocol.Envelope) *Collector {
	return &Collector{
		hostname: hostname,
		out:      out,
	}
}

// wrap creates an envelope from any metric
func (c *Collector) wrap(m protocol.Metric) protocol.Envelope {
	return protocol.Envelope{
		Type:      m.MetricType(),
		Timestamp: time.Now(),
		Hostname:  c.hostname,
		Data:      m,
	}
}

// send handles channel send with context cancellation
func (c *Collector) send(ctx context.Context, m protocol.Metric) {
	select {
	case c.out <- c.wrap(m):
	case <-ctx.Done():
	}
}

// Run executes a collection function at the specified interval
func (c *Collector) Run(ctx context.Context, interval time.Duration, collect CollectFunc) {
	collectAndSend := func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Panic recovered in collector: %v", r)
			}
		}()

		data, err := collect(ctx)
		if err != nil {
			log.Printf("collection failed: %v", err)
			return
		}

		for _, m := range data {
			c.send(ctx, m)
		}
	}

	// Collect Baseline
	collectAndSend()

	// Start ticker
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			collectAndSend()
		}
	}
}
