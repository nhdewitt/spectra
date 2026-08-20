package collector

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

type CollectFunc func(context.Context) ([]protocol.Metric, error)

type Collector struct {
	hostname string
	out      chan<- protocol.Envelope
	logger   *slog.Logger

	mu       sync.Mutex
	reported map[string]bool
}

func New(hostname string, out chan<- protocol.Envelope, logger *slog.Logger) *Collector {
	if logger == nil {
		logger = slog.Default()
	}
	return &Collector{
		hostname: hostname,
		out:      out,
		logger:   logger,
		reported: make(map[string]bool),
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
	if hasNonFinite(m) {
		c.reportNonFinite(m.MetricType())
		return
	}

	select {
	case c.out <- c.wrap(m):
	case <-ctx.Done():
	}
}

// reportNonFinite logs the first rejection of each metric type at Warn and
// every later one at Debug.
//
// An intermittent NaN is one missing sample and needs no attention. A
// persistent one recurs on every collection interval. The first line is what
// distinguishes a broken collector from hardware that simply does not report.
func (c *Collector) reportNonFinite(metricType string) {
	c.mu.Lock()
	first := !c.reported[metricType]
	if first {
		c.reported[metricType] = true
	}
	c.mu.Unlock()

	if first {
		c.logger.Warn("dropping metric with a non-finite value; check the collector for a zero denominator",
			"metric", metricType)
		return
	}
	c.logger.Debug("dropping metric with a non-finite value", "metric", metricType)
}

// Run executes a collection function at the specified interval
func (c *Collector) Run(ctx context.Context, interval time.Duration, collect CollectFunc) {
	collectAndSend := func() {
		defer func() {
			if r := recover(); r != nil {
				c.logger.Error("panic recovered in collector", "panic", r)
			}
		}()

		data, err := collect(ctx)
		if err != nil {
			return
		}

		for _, m := range data {
			if m == nil {
				c.logger.Warn("collector returned a nil metric, skipping")
				continue
			}
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
