package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

const (
	BatchSize    = 100             // If we reach this, send immediately
	SendInterval = 5 * time.Second // Force sending every 5 seconds

	// maxUploadChunk bounds envelopes per request when draining the cache.
	// At observed rates, this is roughly 200KB compressed per request.
	maxUploadChunk = 500
)

// runMetricSender consumes the channel and sends batches via HTTP
func (a *Agent) runMetricSender(ctx context.Context) {
	batch := make([]protocol.Envelope, 0, BatchSize)

	ticker := time.NewTicker(SendInterval)
	defer ticker.Stop()

	flush := func() {
		if len(batch) > 0 {
			a.uploadBatch(ctx, batch)
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

		case <-ctx.Done():
			flush()
			return
		}
	}
}

func (a *Agent) uploadBatch(ctx context.Context, batch []protocol.Envelope) {
	// Honor the backoff window. applyBackoff computes backoffUntil, but until
	// something actually read it every agent kept retrying on the 5s sender
	// cadence throughout an outage.
	if !a.backoffUntil.IsZero() && time.Now().Before(a.backoffUntil) {
		a.cache.Add(batch)
		return
	}

	url := fmt.Sprintf("%s%s", a.Config.BaseURL, a.Config.MetricsPath)

	// Send cached metrics first, in bounded chunks. The cache holds up to
	// defaultMaxCacheSize envelopes, which as a single request is megabytes
	// compressed (too large for the server to bound, and all-or-nothing, so
	// a failure at 90% costs the entire backlog).
	for {
		cached := a.cache.DrainN(maxUploadChunk)
		if len(cached) == 0 {
			break
		}

		switch err := a.postCompressed(ctx, url, cached); {
		case err == nil:
			a.Logger.Debug("sent cached metrics", "count", len(cached), "remaining", a.cache.Len())

		case errors.Is(err, errPayloadEncode), errors.Is(err, errPayloadRejected):
			// Unsendable, so this chunk stays drained. Keep going, the rest of
			// the cache is almost certainly fine.
			a.Logger.Error("dropping cached metrics that cannot be encoded", "count", len(batch), "error", err)

		default:
			a.cache.Requeue(cached)
			a.cache.Add(batch)
			a.applyBackoff()
			a.Logger.Warn("server unreachable",
				"cache_size", a.cache.Len(),
				"retry_in", time.Until(a.backoffUntil).Round(time.Second))
			return
		}
	}

	// Send current batch
	if err := a.postCompressed(ctx, url, batch); err != nil {
		if errors.Is(err, errPayloadEncode) || errors.Is(err, errPayloadRejected) {
			a.Logger.Error("dropping metrics that cannot be encoded", "count", len(batch), "error", err)
			return
		}
		a.cache.Add(batch)
		a.applyBackoff()
		a.Logger.Warn("error sending metrics",
			"error", err,
			"cache_size", a.cache.Len(),
			"retry_in", time.Until(a.backoffUntil).Round(time.Second))
		return
	}

	a.resetBackoff()
}

func (a *Agent) applyBackoff() {
	delay := a.RetryConfig.Delay(a.backoffStep)
	a.backoffStep++

	// Guard the jitter bound. rand.Int64N panics on a non-positive argument, and this
	// is the last line of defense for anything that yields a delay of zero or less
	// (a misconfigured RetryConfig, or an overflow that crashed agents before Delay
	// clamped in the float64 domain).
	if delay <= 0 {
		delay = a.RetryConfig.InitialDelay
	}

	// +/-25% jitter to prevent all agents hammering at the same time on server recovery
	quarter := delay / 4
	if quarter <= 0 {
		a.backoffUntil = time.Now().Add(delay)
		return
	}

	a.backoffUntil = time.Now().Add(delay - quarter + time.Duration(rand.Int64N(int64(2*quarter))))
}

func (a *Agent) resetBackoff() {
	if a.backoffStep > 0 {
		a.Logger.Info("server connection restored")
		a.backoffStep = 0
		a.backoffUntil = time.Time{}
	}
}

// errPayloadEncode marks a payload the agent can never successfully send.
// Retrying is pointless, since a batch that can't be encoded would otherwise
// be re-cached and re-attempted forever.
var errPayloadEncode = errors.New("payload encode failed")

// errPayloadRejected marks a batch the server will never accept, currently a
// 413. Like an encode failure, this is permanent, so caching it would block
// every later flush behind a batch that can never drain.
var errPayloadRejected = errors.New("payload rejected by server")

// compressPayload gzips v into a freshly allocated slice, holding the shared
// gzip writer only for the encode itself.
//
// The lock is released by defer on every path. An earlier version unlocked
// manually after the happy path only, so an encode failure (which encoding/json
// returns for NaN and infinities), values a sensor or a bad denominator can
// procude, left gzipMu held permanently. Both metric uploads and command results
// share this writer, so that deadlocked the agent outright.
func (a *Agent) compressPayload(v any) ([]byte, error) {
	a.gzipMu.Lock()
	defer a.gzipMu.Unlock()

	a.gzipBuf.Reset()
	a.gzipW.Reset(&a.gzipBuf)

	if err := json.NewEncoder(a.gzipW).Encode(v); err != nil {
		return nil, fmt.Errorf("%w: json encode: %w", errPayloadEncode, err)
	}
	if err := a.gzipW.Close(); err != nil {
		return nil, fmt.Errorf("%w: gzip close: %w", errPayloadEncode, err)
	}

	payload := make([]byte, a.gzipBuf.Len())
	copy(payload, a.gzipBuf.Bytes())
	return payload, nil
}

// postCompressed compresses a batch and sends it to the server.
func (a *Agent) postCompressed(ctx context.Context, url string, batch []protocol.Envelope) error {
	payload, err := a.compressPayload(batch)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create request error: %w", err)
	}

	a.setHeaders(req)

	resp, err := a.Client.Do(req)
	if err != nil {
		return fmt.Errorf("http error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusRequestEntityTooLarge {
		return fmt.Errorf("%w: status %d", errPayloadRejected, resp.StatusCode)
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	return nil
}
