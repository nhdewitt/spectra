package agent

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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
