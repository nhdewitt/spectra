package collector

import (
	"context"
	"log"
	"time"

	"github.com/nhdewitt/spectra/metrics"
)

type Collector struct {
	hostname string
	out      chan<- metrics.Envelope
}

func New(hostname string, out chan<- metrics.Envelope) *Collector {
	return &Collector{
		hostname: hostname,
		out:      out,
	}
}

// wrap creates an envelope from any metric
func (c *Collector) wrap(m metrics.Metric) metrics.Envelope {
	return metrics.Envelope{
		Type:      m.MetricType(),
		Timestamp: time.Now(),
		Hostname:  c.hostname,
		Data:      m,
	}
}

// send handles channel send with context cancellation
func (c *Collector) send(ctx context.Context, m metrics.Metric) {
	select {
	case c.out <- c.wrap(m):
	case <-ctx.Done():
	}
}

// Run executes a collection function at the specified interval
func (c *Collector) Run(ctx context.Context, interval time.Duration, collect CollectFunc) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			metricsSlice, err := collect(ctx)
			if err != nil {
				log.Printf("collection error: %v", err)
				continue
			}
			for _, m := range metricsSlice {
				c.send(ctx, m)
			}
		}
	}
}
