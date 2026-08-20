package agent

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func testEnvelope(metricType string) protocol.Envelope {
	return protocol.Envelope{
		Type:      metricType,
		Timestamp: time.Now(),
		Hostname:  "test-host",
		Data:      &protocol.CPUMetric{Usage: 42.0},
	}
}

func TestPostCompressed_Success(t *testing.T) {
	var receivedBytes []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gz, err := gzip.NewReader(r.Body)
		if err != nil {
			t.Errorf("failed to read gzip: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		defer gz.Close()

		receivedBytes, _ = io.ReadAll(gz)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	a := newTestAgentWithLogger()
	a.Config.BaseURL = srv.URL
	a.Config.MetricsPath = "/api/v1/agent/metrics"

	batch := []protocol.Envelope{testEnvelope("cpu"), testEnvelope("cpu")}
	err := a.postCompressed(context.Background(), srv.URL+"/api/v1/agent/metrics", batch)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(receivedBytes) == 0 {
		t.Error("expected non-empty payload")
	}
}

func TestPostCompressed_GzipContent(t *testing.T) {
	var contentEncoding string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentEncoding = r.Header.Get("Content-Encoding")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	a := newTestAgentWithLogger()

	batch := []protocol.Envelope{testEnvelope("cpu")}
	a.postCompressed(context.Background(), srv.URL+"/metrics", batch)

	if contentEncoding != "gzip" {
		t.Errorf("expected Content-Encoding gzip, got %q", contentEncoding)
	}
}

func TestPostCompressed_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	a := newTestAgentWithLogger()

	batch := []protocol.Envelope{testEnvelope("cpu")}
	err := a.postCompressed(context.Background(), srv.URL+"/metrics", batch)

	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestPostCompressed_ServerDown(t *testing.T) {
	a := newTestAgentWithLogger()

	batch := []protocol.Envelope{testEnvelope("cpu")}
	err := a.postCompressed(context.Background(), "http://127.0.0.1:1/metrics", batch)

	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

func TestPostCompressed_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := newTestAgentWithLogger()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	batch := []protocol.Envelope{testEnvelope("cpu")}
	err := a.postCompressed(ctx, srv.URL+"/metrics", batch)

	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestPostCompressed_SetsAuthHeaders(t *testing.T) {
	var agentID, agentSecret string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agentID = r.Header.Get("X-Agent-ID")
		agentSecret = r.Header.Get("X-Agent-Secret")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	a := newTestAgentWithLogger()

	batch := []protocol.Envelope{testEnvelope("cpu")}
	a.postCompressed(context.Background(), srv.URL+"/metrics", batch)

	if agentID != a.Identity.ID {
		t.Errorf("expected X-Agent-ID %q, got %q", a.Identity.ID, agentID)
	}
	if agentSecret != a.Identity.Secret {
		t.Errorf("expected X-Agent-Secret %q, got %q", a.Identity.Secret, agentSecret)
	}
}

func TestPostCompressed_Status299OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted) // 202
	}))
	defer srv.Close()

	a := newTestAgentWithLogger()

	err := a.postCompressed(context.Background(), srv.URL+"/metrics", []protocol.Envelope{testEnvelope("cpu")})
	if err != nil {
		t.Fatalf("expected no error for 202, got: %v", err)
	}
}

func TestPostCompressed_Status300Fails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusMovedPermanently) // 301
	}))
	defer srv.Close()

	a := newTestAgentWithLogger()

	err := a.postCompressed(context.Background(), srv.URL+"/metrics", []protocol.Envelope{testEnvelope("cpu")})
	if err == nil {
		t.Fatal("expected error for 301 response")
	}
}

func TestUploadBatch_Success(t *testing.T) {
	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		// drain the body
		io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	a := newTestAgentWithLogger()
	a.Config.BaseURL = srv.URL
	a.Config.MetricsPath = "/api/v1/agent/metrics"

	batch := []protocol.Envelope{testEnvelope("cpu"), testEnvelope("memory")}
	a.uploadBatch(context.Background(), batch)

	if callCount.Load() != 1 {
		t.Errorf("expected 1 POST, got %d", callCount.Load())
	}
}

func TestUploadBatch_CachesOnFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	a := newTestAgentWithLogger()
	a.Config.BaseURL = srv.URL
	a.Config.MetricsPath = "/api/v1/agent/metrics"

	batch := []protocol.Envelope{testEnvelope("cpu"), testEnvelope("cpu")}
	a.uploadBatch(context.Background(), batch)

	if a.cache.Len() != 2 {
		t.Errorf("expected 2 cached envelopes, got %d", a.cache.Len())
	}
}

func TestUploadBatch_DrainsCacheFirst(t *testing.T) {
	var calls []int // track envelope counts per call
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gz, _ := gzip.NewReader(r.Body)
		var batch []protocol.Envelope
		json.NewDecoder(gz).Decode(&batch)
		gz.Close()
		calls = append(calls, len(batch))
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	a := newTestAgentWithLogger()
	a.Config.BaseURL = srv.URL
	a.Config.MetricsPath = "/api/v1/agent/metrics"

	a.cache.Add([]protocol.Envelope{testEnvelope("cpu"), testEnvelope("cpu"), testEnvelope("cpu")})

	batch := []protocol.Envelope{testEnvelope("memory")}
	a.uploadBatch(context.Background(), batch)

	if len(calls) != 2 {
		t.Fatalf("expected 2 POST calls (cached + current), got %d", len(calls))
	}
	if calls[0] != 3 {
		t.Errorf("first call should send 3 cached envelopes, got %d", calls[0])
	}
	if calls[1] != 1 {
		t.Errorf("second call should send 1 current envelope, got %d", calls[1])
	}
}

func TestUploadBatch_CachesDrainFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	a := newTestAgentWithLogger()
	a.Config.BaseURL = srv.URL
	a.Config.MetricsPath = "/api/v1/agent/metrics"

	// Pre-populate cache with 2
	a.cache.Add([]protocol.Envelope{testEnvelope("cpu"), testEnvelope("cpu")})

	// Send batch of 1
	batch := []protocol.Envelope{testEnvelope("memory")}
	a.uploadBatch(context.Background(), batch)

	// Both cached (2) and current (1) should be re-cached
	if a.cache.Len() != 3 {
		t.Errorf("expected 3 cached envelopes (2 old + 1 new), got %d", a.cache.Len())
	}
}

func TestUploadBatch_EmptyCache(t *testing.T) {
	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	a := newTestAgentWithLogger()
	a.Config.BaseURL = srv.URL
	a.Config.MetricsPath = "/api/v1/agent/metrics"

	batch := []protocol.Envelope{testEnvelope("cpu")}
	a.uploadBatch(context.Background(), batch)

	if callCount.Load() != 1 {
		t.Errorf("expected 1 POST, got %d", callCount.Load())
	}
}

func TestRunMetricSender_FlushesOnContextCancel(t *testing.T) {
	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	a := newTestAgentWithLogger()
	a.Config.BaseURL = srv.URL
	a.Config.MetricsPath = "/api/v1/agent/metrics"
	ch := make(chan protocol.Envelope, BatchSize+10)
	a.metricsCh = ch

	for range BatchSize {
		ch <- testEnvelope("cpu")
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(500 * time.Millisecond)
		cancel()
	}()

	a.runMetricSender(ctx)

	if callCount.Load() == 0 {
		t.Error("expected at least one flush on context cancel")
	}
}

func TestRunMetricSender_FlushesOnChannelClose(t *testing.T) {
	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	a := newTestAgentWithLogger()
	a.Config.BaseURL = srv.URL
	a.Config.MetricsPath = "/api/v1/agent/metrics"
	// Replace channel so we control it
	ch := make(chan protocol.Envelope, 10)
	a.metricsCh = ch

	ctx := context.Background()

	go func() {
		ch <- testEnvelope("cpu")
		ch <- testEnvelope("cpu")
		time.Sleep(50 * time.Millisecond)
		close(ch)
	}()

	a.runMetricSender(ctx)

	if callCount.Load() == 0 {
		t.Error("expected at least one flush on channel close")
	}
}

func TestRunMetricSender_BatchSizeFlush(t *testing.T) {
	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	a := newTestAgentWithLogger()
	a.Config.BaseURL = srv.URL
	a.Config.MetricsPath = "/api/v1/agent/metrics"
	ch := make(chan protocol.Envelope, BatchSize+10)
	a.metricsCh = ch

	// Fill past BatchSize
	for range BatchSize + 1 {
		ch <- testEnvelope("cpu")
	}

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()

	a.runMetricSender(ctx)

	// Should have at least 1 batch-size flush plus the remainder
	if callCount.Load() < 1 {
		t.Errorf("expected at least 1 flush for %d envelopes, got %d calls", BatchSize+1, callCount.Load())
	}
}

// nanEnvelope carries a value encoding/json refuses to marshal. A collector
// producing NaN is not hypothetical: any rate or percentage with a zero
// denominator gets there, as does a misbehaving sensor.
func nanEnvelope() protocol.Envelope {
	return protocol.Envelope{
		Type:      "cpu",
		Timestamp: time.Now(),
		Hostname:  "test-host",
		Data:      &protocol.CPUMetric{Usage: math.NaN()},
	}
}

func TestCompressPayload_EncodeErrorReleasesLock(t *testing.T) {
	a := newTestAgentWithLogger()

	_, err := a.compressPayload([]protocol.Envelope{nanEnvelope()})
	if err == nil {
		t.Fatal("expected an encode error for NaN")
	}
	if !errors.Is(err, errPayloadEncode) {
		t.Errorf("error: got %v, want it to wrap errPayloadEncode", err)
	}

	if !a.gzipMu.TryLock() {
		t.Fatal("gzipMu still held after a failed encode: every later metric upload and command result would block forever")
	}
	a.gzipMu.Unlock()
}

func TestCompressPayload_UsableAfterEncodeError(t *testing.T) {
	a := newTestAgentWithLogger()

	if _, err := a.compressPayload([]protocol.Envelope{nanEnvelope()}); err == nil {
		t.Fatal("expected an encode error for NaN")
	}

	// The failed encode left a partially written gzip stream behind; the next
	// call must reset it rather than append to it.
	payload, err := a.compressPayload([]protocol.Envelope{testEnvelope("cpu")})
	if err != nil {
		t.Fatalf("compress after a failed encode: %v", err)
	}

	gz, err := gzip.NewReader(bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("payload is not valid gzip: %v", err)
	}
	defer gz.Close()

	// Envelope.Data is a protocol.Metric interface, so an Envelope does not
	// round-trip through Decode. Decode structurally instead.
	var decoded []map[string]any
	if err := json.NewDecoder(gz).Decode(&decoded); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if len(decoded) != 1 {
		t.Fatalf("decoded envelopes: got %d, want 1", len(decoded))
	}
	if decoded[0]["type"] != "cpu" {
		t.Errorf("decoded envelope type: got %v, want cpu", decoded[0]["type"])
	}
}

func TestUploadBatch_DropsUnencodableBatch(t *testing.T) {
	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	a := newTestAgentWithLogger()
	a.Config.BaseURL = srv.URL
	a.Config.MetricsPath = "/api/v1/agent/metrics"

	a.uploadBatch(context.Background(), []protocol.Envelope{nanEnvelope()})

	if callCount.Load() != 0 {
		t.Errorf("POST calls: got %d, want 0", callCount.Load())
	}
	if a.cache.Len() != 0 {
		t.Errorf("cached envelopes: got %d, want 0: an unencodable batch fails identically on every retry and would block the cache forever", a.cache.Len())
	}
	if a.backoffStep != 0 {
		t.Errorf("backoffStep: got %d, want 0: an encode failure is not a transport failure", a.backoffStep)
	}
}

func TestUploadBatch_DropsUnencodableCacheThenSendsBatch(t *testing.T) {
	var sent []int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gz, _ := gzip.NewReader(r.Body)
		var batch []map[string]any
		json.NewDecoder(gz).Decode(&batch)
		gz.Close()
		sent = append(sent, len(batch))
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	a := newTestAgentWithLogger()
	a.Config.BaseURL = srv.URL
	a.Config.MetricsPath = "/api/v1/agent/metrics"

	a.cache.Add([]protocol.Envelope{nanEnvelope()})

	a.uploadBatch(context.Background(), []protocol.Envelope{testEnvelope("cpu")})

	if len(sent) != 1 {
		t.Fatalf("POST calls: got %d, want 1 (poisoned cache dropped, current batch still sent)", len(sent))
	}
	if sent[0] != 1 {
		t.Errorf("envelopes in the surviving call: got %d, want 1", sent[0])
	}
	if a.cache.Len() != 0 {
		t.Errorf("cached envelopes: got %d, want 0", a.cache.Len())
	}
}

func TestUploadBatch_SkipsNetworkDuringBackoff(t *testing.T) {
	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	a := newTestAgentWithLogger()
	a.Config.BaseURL = srv.URL
	a.Config.MetricsPath = "/api/v1/agent/metrics"
	a.backoffUntil = time.Now().Add(time.Minute)

	batch := []protocol.Envelope{testEnvelope("cpu"), testEnvelope("memory")}
	a.uploadBatch(context.Background(), batch)

	if callCount.Load() != 0 {
		t.Errorf("POST calls: got %d, want 0 while inside the backoff window", callCount.Load())
	}
	if a.cache.Len() != 2 {
		t.Errorf("cached envelopes: got %d, want 2", a.cache.Len())
	}
}

func TestUploadBatch_ResumesWhenBackoffExpires(t *testing.T) {
	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	a := newTestAgentWithLogger()
	a.Config.BaseURL = srv.URL
	a.Config.MetricsPath = "/api/v1/agent/metrics"
	a.backoffUntil = time.Now().Add(-time.Second)

	a.uploadBatch(context.Background(), []protocol.Envelope{testEnvelope("cpu")})

	if callCount.Load() != 1 {
		t.Errorf("POST calls: got %d, want 1 once the backoff window has passed", callCount.Load())
	}
}

// TestUploadBatch_FailureSuppressesTheNextFlush is the end-to-end version:
// applyBackoff computed a delay for a long time, but nothing read it, so every
// agent kept retrying on the 5s sender cadence right through an outage.
func TestUploadBatch_FailureSuppressesTheNextFlush(t *testing.T) {
	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	a := newTestAgentWithLogger()
	a.Config.BaseURL = srv.URL
	a.Config.MetricsPath = "/api/v1/agent/metrics"

	a.uploadBatch(context.Background(), []protocol.Envelope{testEnvelope("cpu")})
	if callCount.Load() != 1 {
		t.Fatalf("POST calls after the first flush: got %d, want 1", callCount.Load())
	}
	if a.backoffStep != 1 {
		t.Fatalf("backoffStep: got %d, want 1", a.backoffStep)
	}

	// DefaultRetryConfig's InitialDelay is 1s, so a flush issued immediately
	// after the failure falls inside the window.
	a.uploadBatch(context.Background(), []protocol.Envelope{testEnvelope("memory")})
	if callCount.Load() != 1 {
		t.Errorf("POST calls after the second flush: got %d, want 1: the backoff window was ignored", callCount.Load())
	}
	if a.cache.Len() != 2 {
		t.Errorf("cached envelopes: got %d, want 2", a.cache.Len())
	}
}

// --- Chunked cache drain ---

// TestUploadBatch_DrainsCacheInChunks pins the loop. A full cache is
// defaultMaxCacheSize envelopes, which as one request is megabytes compressed;
// if this silently reverts to a single Drain, any server-side body limit
// becomes a cliff that a backlogged agent can never get past.
func TestUploadBatch_DrainsCacheInChunks(t *testing.T) {
	var sizes []int
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gz, _ := gzip.NewReader(r.Body)
		var batch []map[string]any
		json.NewDecoder(gz).Decode(&batch)
		gz.Close()

		mu.Lock()
		sizes = append(sizes, len(batch))
		mu.Unlock()

		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	a := newTestAgentWithLogger()
	a.Config.BaseURL = srv.URL
	a.Config.MetricsPath = "/api/v1/agent/metrics"

	// Two full chunks plus a remainder, then the current batch.
	a.cache.Add(makeEnvelopes(maxUploadChunk*2 + 30))

	a.uploadBatch(context.Background(), []protocol.Envelope{testEnvelope("cpu")})

	mu.Lock()
	defer mu.Unlock()

	if len(sizes) != 4 {
		t.Fatalf("POST calls: got %d, want 4 (3 cache chunks + the current batch): %v", len(sizes), sizes)
	}
	if sizes[0] != maxUploadChunk || sizes[1] != maxUploadChunk {
		t.Errorf("chunk sizes: got %d and %d, want %d each", sizes[0], sizes[1], maxUploadChunk)
	}
	if sizes[2] != 30 {
		t.Errorf("final cache chunk: got %d, want 30", sizes[2])
	}
	if sizes[3] != 1 {
		t.Errorf("current batch: got %d envelopes, want 1", sizes[3])
	}
	if a.cache.Len() != 0 {
		t.Errorf("cache after a clean drain: got %d, want 0", a.cache.Len())
	}
}

// TestUploadBatch_RequeuesFailedChunk covers a mid-drain outage: chunks already
// accepted stay accepted, the failed chunk goes back, and the current batch is
// added behind it.
func TestUploadBatch_RequeuesFailedChunk(t *testing.T) {
	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		if callCount.Add(1) == 1 {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	a := newTestAgentWithLogger()
	a.Config.BaseURL = srv.URL
	a.Config.MetricsPath = "/api/v1/agent/metrics"

	a.cache.Add(makeEnvelopes(maxUploadChunk * 2))

	a.uploadBatch(context.Background(), []protocol.Envelope{testEnvelope("cpu")})

	if callCount.Load() != 2 {
		t.Fatalf("POST calls: got %d, want 2 (one accepted, one failed, then stop)", callCount.Load())
	}

	// Second chunk requeued (500) + the current batch (1). The first chunk was
	// accepted and must not come back.
	if a.cache.Len() != maxUploadChunk+1 {
		t.Errorf("cache size: got %d, want %d", a.cache.Len(), maxUploadChunk+1)
	}
	if a.backoffStep != 1 {
		t.Errorf("backoffStep: got %d, want 1", a.backoffStep)
	}
}

// TestUploadBatch_UnencodableChunkDoesNotStopTheDrain confirms the blast radius
// of a NaN is now one chunk rather than the whole backlog.
func TestUploadBatch_UnencodableChunkDoesNotStopTheDrain(t *testing.T) {
	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	a := newTestAgentWithLogger()
	a.Config.BaseURL = srv.URL
	a.Config.MetricsPath = "/api/v1/agent/metrics"

	// One poisoned envelope in the first chunk, a clean second chunk behind it.
	first := makeEnvelopes(maxUploadChunk - 1)
	first = append(first, nanEnvelope())
	a.cache.Add(first)
	a.cache.Add(makeEnvelopes(10))

	a.uploadBatch(context.Background(), []protocol.Envelope{testEnvelope("cpu")})

	// The poisoned chunk never reaches the network; the rest does.
	if callCount.Load() != 2 {
		t.Errorf("POST calls: got %d, want 2 (clean cache chunk + current batch)", callCount.Load())
	}
	if a.cache.Len() != 0 {
		t.Errorf("cache: got %d, want 0", a.cache.Len())
	}
}

// TestUploadBatch_DropsBatchRejectedAs413 covers the counterpart to the
// server's body limit. The agent retries anything that fails, so without this a
// batch the server will never accept sits at the head of the cache blocking
// every healthy flush behind it, forever.
func TestUploadBatch_DropsBatchRejectedAs413(t *testing.T) {
	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusRequestEntityTooLarge)
	}))
	defer srv.Close()

	a := newTestAgentWithLogger()
	a.Config.BaseURL = srv.URL
	a.Config.MetricsPath = "/api/v1/agent/metrics"

	a.uploadBatch(context.Background(), []protocol.Envelope{testEnvelope("cpu")})

	if callCount.Load() != 1 {
		t.Errorf("POST calls: got %d, want 1", callCount.Load())
	}
	if a.cache.Len() != 0 {
		t.Errorf("cached envelopes: got %d, want 0: a rejected batch fails identically on every retry", a.cache.Len())
	}
	if a.backoffStep != 0 {
		t.Errorf("backoffStep: got %d, want 0: a rejected batch is not a transport failure", a.backoffStep)
	}
}

func TestUploadBatch_ServerErrorStillRetries(t *testing.T) {
	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	a := newTestAgentWithLogger()
	a.Config.BaseURL = srv.URL
	a.Config.MetricsPath = "/api/v1/agent/metrics"

	a.uploadBatch(context.Background(), []protocol.Envelope{testEnvelope("cpu")})

	// A 500 is transient, unlike a 413: the batch is kept and retried.
	if a.cache.Len() != 1 {
		t.Errorf("cached envelopes: got %d, want 1", a.cache.Len())
	}
	if a.backoffStep != 1 {
		t.Errorf("backoffStep: got %d, want 1", a.backoffStep)
	}
}

// --- Backoff overflow (crash observed on raspi-1, 2026-08-17) ---

// TestRetryConfigDelay_NeverOverflows walks past the attempt count where the
// float64 accumulator exceeds MaxInt64. Before the clamp moved into the float
// domain, the conversion produced a negative duration on armv6, the cap did not
// fire, and applyBackoff panicked in rand.Int64N.
func TestRetryConfigDelay_NeverOverflows(t *testing.T) {
	rc := DefaultRetryConfig()

	for attempt := range 200 {
		delay := rc.Delay(attempt)

		if delay <= 0 {
			t.Fatalf("attempt %d: delay is %v, want positive", attempt, delay)
		}
		if delay > rc.MaxDelay {
			t.Fatalf("attempt %d: delay is %v, want no more than %v", attempt, delay, rc.MaxDelay)
		}
	}
}

func TestRetryConfigDelay_RampsThenClamps(t *testing.T) {
	rc := DefaultRetryConfig()

	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{0, time.Second},
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{3, 8 * time.Second},
		{4, 16 * time.Second},
		{5, rc.MaxDelay}, // 32s would exceed the 30s cap
		{40, rc.MaxDelay},
	}

	for _, tc := range tests {
		if got := rc.Delay(tc.attempt); got != tc.want {
			t.Errorf("Delay(%d): got %v, want %v", tc.attempt, got, tc.want)
		}
	}
}

// TestApplyBackoff_SurvivesLongOutage is the regression test for the crash
// itself: it drives applyBackoff past the overflow point and requires both that
// it does not panic and that every window it sets is usable.
func TestApplyBackoff_SurvivesLongOutage(t *testing.T) {
	a := newTestAgentWithLogger()

	for i := range 200 {
		a.applyBackoff()

		wait := time.Until(a.backoffUntil)
		if wait <= 0 {
			t.Fatalf("step %d: backoff window is %v, want positive", i, wait)
		}
		if wait > 2*a.RetryConfig.MaxDelay {
			t.Fatalf("step %d: backoff window is %v, want no more than %v", i, wait, 2*a.RetryConfig.MaxDelay)
		}
	}
}

// TestApplyBackoff_ToleratesDegenerateConfig covers the guard directly: a
// sub-4ns delay makes the jitter quarter zero, and rand.Int64N(0) panics too.
func TestApplyBackoff_ToleratesDegenerateConfig(t *testing.T) {
	a := newTestAgentWithLogger()
	a.RetryConfig = RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 1,
		MaxDelay:     2,
		Multiplier:   2.0,
	}

	for range 10 {
		a.applyBackoff()
	}
}
