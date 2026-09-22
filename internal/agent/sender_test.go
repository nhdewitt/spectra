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

func TestUploadBatch_SendsCacheAndBatchInOneRequest(t *testing.T) {
	var calls []int
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

	// The current batch goes into the cache and drains with it, so a backlog
	// under maxUploadChunk is one request, not two.
	if len(calls) != 1 {
		t.Fatalf("POST calls: got %d, want 1 (cache and batch travel together): %v", len(calls), calls)
	}
	if calls[0] != 4 {
		t.Errorf("envelopes in the request: got %d, want 4", calls[0])
	}
	if a.cache.Len() != 0 {
		t.Errorf("cache after a clean send: got %d, want 0", a.cache.Len())
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

	// Exactly BatchSize envelopes, so all of them are parked in the cache and
	// batch is empty by the time the cancel lands. The flush still has to send
	// them: draining cannot depend on new metrics still arriving.
	if callCount.Load() == 0 {
		t.Error("expected a flush on context cancel with an empty batch and a full cache")
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

	// Reaching BatchSize parks the accumulator in the cache without sending, so
	// the only network call is the one the cancel triggers — the whole point of
	// pacing is that a burst of buffered envelopes does not become a burst of
	// requests.
	if got := callCount.Load(); got != 1 {
		t.Errorf("POST calls: got %d, want 1 (BatchSize must stash, not send)", got)
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

// TestUploadBatch_PoisonedChunkTakesTheCurrentBatchWithIt pins a deliberate
// regression from merging the cache and the current batch into one request. A
// single unencodable envelope fails the whole chunk, and the chunk now contains
// live data. The blast radius is still one chunk and the next cycle is clean,
// so the gap is one SendInterval.
func TestUploadBatch_PoisonedChunkTakesTheCurrentBatchWithIt(t *testing.T) {
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

	if len(sent) != 0 {
		t.Fatalf("POST calls: got %d, want 0 (the chunk never encodes)", len(sent))
	}
	if a.cache.Len() != 0 {
		t.Errorf("cached envelopes: got %d, want 0: the chunk must not be requeued", a.cache.Len())
	}
	if a.backoffStep != 0 {
		t.Errorf("backoffStep: got %d, want 0: an encode failure is not a transport failure", a.backoffStep)
	}

	// The next cycle is clean and gets through.
	a.uploadBatch(context.Background(), []protocol.Envelope{testEnvelope("memory")})
	if len(sent) != 1 || sent[0] != 1 {
		t.Errorf("recovery send: got %v, want one call of 1 envelope", sent)
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
func TestUploadBatch_SendsOneChunkPerCycle(t *testing.T) {
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

	a.cache.Add(makeEnvelopes(maxUploadChunk*2 + 30))

	a.uploadBatch(context.Background(), []protocol.Envelope{testEnvelope("cpu")})

	mu.Lock()
	if len(sizes) != 1 || sizes[0] != maxUploadChunk {
		mu.Unlock()
		t.Fatalf("first cycle: got %v, want one call of %d", sizes, maxUploadChunk)
	}
	mu.Unlock()

	// One chunk out per cycle against whatever came in: 1031 - 500 = 531.
	if want := maxUploadChunk + 31; a.cache.Len() != want {
		t.Errorf("cache after one cycle: got %d, want %d", a.cache.Len(), want)
	}

	// Three more cycles clear it.
	for range 3 {
		a.uploadBatch(context.Background(), nil)
	}
	if a.cache.Len() != 0 {
		t.Errorf("cache after four cycles: got %d, want 0", a.cache.Len())
	}
	mu.Lock()
	defer mu.Unlock()
	if len(sizes) != 3 {
		t.Errorf("POST calls: got %d, want 3 (500, 500, 31): %v", len(sizes), sizes)
	}
}

// TestUploadBatch_RequeuesFailedChunk covers an outage part way through a
// catch-up: the chunk already accepted stays accepted, the failed one goes back
// to the tail it came from.
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

	// First cycle succeeds and clears backoff; second fails and requeues.
	a.uploadBatch(context.Background(), []protocol.Envelope{testEnvelope("cpu")})
	a.uploadBatch(context.Background(), nil)

	if callCount.Load() != 2 {
		t.Fatalf("POST calls: got %d, want 2", callCount.Load())
	}
	// 1001 in, 500 accepted, the next 500 drained and put back.
	if want := maxUploadChunk + 1; a.cache.Len() != want {
		t.Errorf("cache size: got %d, want %d", a.cache.Len(), want)
	}
	if a.backoffStep != 1 {
		t.Errorf("backoffStep: got %d, want 1", a.backoffStep)
	}
}

// TestUploadBatch_UnencodableChunkDoesNotBlockTheBacklog confirms a NaN costs
// one chunk rather than wedging the cache: the poisoned chunk is dropped, not
// requeued, so the next cycle reaches the envelopes behind it.
func TestUploadBatch_UnencodableChunkDoesNotBlockTheBacklog(t *testing.T) {
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

	// A poisoned envelope at the head, where the first drain will find it, with
	// clean envelopes queued behind it.
	a.cache.Add([]protocol.Envelope{nanEnvelope()})
	a.cache.Add(makeEnvelopes(10))

	a.uploadBatch(context.Background(), nil)
	if callCount.Load() != 0 {
		t.Fatalf("POST calls: got %d, want 0 (poisoned chunk never encodes)", callCount.Load())
	}
	if a.cache.Len() != 0 {
		t.Errorf("cache: got %d, want 0 (the whole chunk is dropped)", a.cache.Len())
	}

	// The cache is usable afterwards rather than wedged.
	a.uploadBatch(context.Background(), []protocol.Envelope{testEnvelope("cpu")})
	if callCount.Load() != 1 {
		t.Errorf("POST calls after recovery: got %d, want 1", callCount.Load())
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

// --- Compression level ---
// The shared writer used to be gzip.NewWriter, which is DefaultCompression.
// On a single-core Pi compression was ~23% of agent CPU, and the level-6
// deflate state sat at roughly 900kB resident against a 48MiB ceiling.

// benchBatch builds a batch shaped like a real metric send: repeated envelopes
// of mixed types, which is what makes the ratio difference small.
func benchBatch(n int) []protocol.Envelope {
	batch := make([]protocol.Envelope, 0, n)
	for i := range n {
		switch i % 3 {
		case 0:
			batch = append(batch, testEnvelope("cpu"))
		case 1:
			batch = append(batch, protocol.Envelope{
				Type:      "memory",
				Timestamp: time.Now(),
				Hostname:  "test-host",
				Data: &protocol.MemoryMetric{
					Total:     16 << 30,
					Used:      4 << 30,
					Available: 12 << 30,
					UsedPct:   25.0,
					SwapTotal: 2 << 30,
					SwapUsed:  128 << 20,
					SwapPct:   6.25,
				},
			})
		default:
			batch = append(batch, protocol.Envelope{
				Type:      "disk",
				Timestamp: time.Now(),
				Hostname:  "test-host",
				Data: &protocol.DiskMetric{
					Device:     "/dev/sda1",
					Mountpoint: "/",
					Filesystem: "ext4",
					Type:       "ssd",
					Total:      250 << 30,
					Used:       130 << 30,
					Available:  120 << 30,
					UsedPct:    52.0,
				},
			})
		}
	}
	return batch
}

func benchCompressAtLevel(b *testing.B, level int, n int) {
	b.Helper()

	a := New(Config{BaseURL: "http://localhost:8080", Hostname: "test-host"})
	zw, err := gzip.NewWriterLevel(io.Discard, level)
	if err != nil {
		b.Fatalf("NewWriterLevel(%d): %v", level, err)
	}
	a.gzipW = zw

	batch := benchBatch(n)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		if _, err := a.compressPayload(batch); err != nil {
			b.Fatalf("compressPayload: %v", err)
		}
	}

	b.StopTimer()

	// Ratio is the other half of the trade: a level that is faster but gives
	// up a lot of compression costs more in upload time than it saves in CPU.
	//
	// Reported after the loop because ResetTimer deletes user metrics, so
	// anything reported before it never reaches the output.
	payload, err := a.compressPayload(batch)
	if err != nil {
		b.Fatalf("compressPayload: %v", err)
	}
	raw, err := json.Marshal(batch)
	if err != nil {
		b.Fatalf("marshal: %v", err)
	}
	b.ReportMetric(float64(len(raw))/float64(len(payload)), "ratio")
	b.ReportMetric(float64(len(payload)), "wire-bytes")
}

func BenchmarkCompressPayload_BestSpeed_50(b *testing.B) {
	benchCompressAtLevel(b, gzip.BestSpeed, 50)
}

func BenchmarkCompressPayload_Default_50(b *testing.B) {
	benchCompressAtLevel(b, gzip.DefaultCompression, 50)
}

func BenchmarkCompressPayload_BestSpeed_500(b *testing.B) {
	benchCompressAtLevel(b, gzip.BestSpeed, 500)
}

func BenchmarkCompressPayload_Default_500(b *testing.B) {
	benchCompressAtLevel(b, gzip.DefaultCompression, 500)
}

// The compressor is built once and held for the life of the process, so its
// resident state is a permanent cost rather than a per-send one.
func BenchmarkNewGzipWriter_BestSpeed(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sink = newGzipWriter(io.Discard)
	}
}

func BenchmarkNewGzipWriter_Default(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		zw, err := gzip.NewWriterLevel(io.Discard, gzip.DefaultCompression)
		if err != nil {
			b.Fatal(err)
		}
		sink = zw
	}
}

var sink *gzip.Writer

func TestNewGzipWriter_UsesBestSpeed(t *testing.T) {
	a := New(Config{BaseURL: "http://localhost:8080", Hostname: "test-host"})

	batch := []protocol.Envelope{testEnvelope("cpu")}
	got, err := a.compressPayload(batch)
	if err != nil {
		t.Fatalf("compressPayload: %v", err)
	}

	var want bytes.Buffer
	zw, err := gzip.NewWriterLevel(&want, gzip.BestSpeed)
	if err != nil {
		t.Fatalf("NewWriterLevel: %v", err)
	}
	if err := json.NewEncoder(zw).Encode(batch); err != nil {
		t.Fatalf("encode: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	if !bytes.Equal(got, want.Bytes()) {
		t.Error("agent compressor does not match a BestSpeed writer")
	}
}

// Whatever the level, the gzip stream has to decompress to the batch the
// server will decode. Envelope.Data is an interface, so the envelopes are
// left raw here -- dispatching them by type is the server's job.
func TestCompressPayload_RoundTrips(t *testing.T) {
	a := New(Config{BaseURL: "http://localhost:8080", Hostname: "test-host"})
	batch := benchBatch(10)

	payload, err := a.compressPayload(batch)
	if err != nil {
		t.Fatalf("compressPayload: %v", err)
	}

	zr, err := gzip.NewReader(bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	defer zr.Close()

	var back []struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(zr).Decode(&back); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(back) != len(batch) {
		t.Fatalf("round trip gave %d envelopes, want %d", len(back), len(batch))
	}
	for i, env := range back {
		if env.Type != batch[i].Type {
			t.Errorf("envelope %d type = %q, want %q", i, env.Type, batch[i].Type)
		}
		if len(env.Data) == 0 {
			t.Errorf("envelope %d carries no data", i)
		}
	}
}
