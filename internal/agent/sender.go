package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"runtime"
	"runtime/metrics"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

const (
	BatchSize    = 100             // Park the accumulator in the cache at this size
	SendInterval = 5 * time.Second // Force sending every 5 seconds

	// shutdownFlushTimeout bounds the final flush, which runs on a context of its
	// own because the one runMetricSender was started with is already cancelled by
	// then. Keep it inside systemd's TimeoutStopSec.
	shutdownFlushTimeout = 5 * time.Second

	// maxUploadChunk bounds envelopes per request, and with one chunk per send
	// cycle it doubles as the catch-up rate: maxUploadChunk out per SendInterval
	// against whatever came in. Roughly 200KB compressed per request.
	maxUploadChunk = 500
)

// runMetricSender consumes the channel and sends batches via HTTP.
func (a *Agent) runMetricSender(ctx context.Context) {
	batch := make([]protocol.Envelope, 0, BatchSize)

	ticker := time.NewTicker(SendInterval)
	defer ticker.Stop()

	flush := func(ctx context.Context) {
		if len(batch) > 0 || a.cache.Len() > 0 {
			a.uploadBatch(ctx, batch)
			batch = batch[:0]
		}
	}

	for {
		select {
		case envelope, ok := <-a.metricsCh:
			if !ok {
				flush(ctx)
				return
			}
			batch = append(batch, envelope)
			if len(batch) >= BatchSize {
				// Park in the cache rather than sending. Request size is bounded
				// by maxUploadChunk at the drain now, so the only job left here
				// is keeping batch from growing between ticks. Sending would
				// defeat the pacing. After a send blocks on a timeout, metricsCh
				// holds everything the collectors buffered and draining that
				// fires several chunks back to back at exactly the moment the
				// whole fleet is reconnecting.
				a.cache.Add(batch)
				batch = batch[:0]
			}

		case <-ticker.C:
			flush(ctx)

		case <-ctx.Done():
			// ctx is cancelled by definition here, and a send on it fails before
			// the request leaves the process. The final flush needs a live context
			// or everything collected since the last tick is lost.
			flushCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownFlushTimeout)
			flush(flushCtx)
			cancel()
			return
		}
	}
}

func (a *Agent) uploadBatch(ctx context.Context, batch []protocol.Envelope) {
	// During a backlog, the current batch queues behind it. Once the backlog
	// clears, the thing being drained is the current batch. The cost is that
	// live data is delayed for the length of the catch-up (roughly 75s after
	// an hour-long outage, 30m for a full cache).
	a.cache.Add(batch)

	// Honor the backoff window. applyBackoff computes backoffUntil, but until
	// something actually read it every agent kept retrying on the 5s sender
	// cadence throughout an outage.
	if !a.backoffUntil.IsZero() && time.Now().Before(a.backoffUntil) {
		return
	}

	// One chunk per send cycle rather than draining to empty. At maxUploadChunk
	// out against roughly ten envelopes in, a backlog still clears quickly, but
	// recovery costs each agent one extra request per SendInterval instead of
	// hundreds back to back. Backoff jitter only spreads the start of a recovery,
	// so without this the whole fleet lands on the server at once.
	chunk := a.cache.DrainN(maxUploadChunk)
	if len(chunk) == 0 {
		return
	}

	url := fmt.Sprintf("%s%s", a.Config.BaseURL, a.Config.MetricsPath)

	switch err := a.postCompressed(ctx, url, chunk); {
	case err == nil:
		a.resetBackoff()
		if remaining := a.cache.Len(); remaining > 0 {
			a.Logger.Debug("catching up", "sent", len(chunk), "remaining", remaining)
		}

	case errors.Is(err, errPayloadEncode), errors.Is(err, errPayloadRejected):
		// Unsendable. Fails identically on every retry, so it stays drained.
		// Not a transport failure, so the backoff is left alone. Costs the whole
		// chunk it travels in. The next cycle is clean, so the gap is one
		// SendInterval.
		a.Logger.Error("dropping metrics that cannot be encoded", "count", len(chunk), "error", err)

	default:
		a.cache.Requeue(chunk)
		a.applyBackoff()
		a.Logger.Warn("server unreachable",
			"error", err,
			"cache_size", a.cache.Len(),
			"cache_bytes", a.cache.Bytes(),
			"heap_bytes", a.heapBytes(),
			"retry_in", time.Until(a.backoffUntil).Round(time.Second))
	}
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

const heapCalibrateEnv = "SPECTRA_HEAP_CALIBRATE"

// heapBytes reports live heap opbjects.
// func heapBytes() uint64 {
// 	sample := []metrics.Sample{{Name: "/memory/classes/heap/objects:bytes"}}
// 	metrics.Read(sample)

// 	if sample[0].Value.Kind() != metrics.KindUint64 {
// 		return 0
// 	}
// 	return sample[0].Value.Uint64()
// }

func (a *Agent) heapBytes() uint64 {
	if a.calibrateHeap {
		runtime.GC()
		runtime.GC()
	}

	sample := []metrics.Sample{{Name: "/memory/classes/heap/objects:bytes"}}
	metrics.Read(sample)

	if sample[0].Value.Kind() != metrics.KindUint64 {
		return 0
	}
	return sample[0].Value.Uint64()
}
