package collector

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

// Mock Metric implementation for testing
type mockMetric struct {
	Value int
}

func (m mockMetric) MetricType() string {
	return "MOCK_METRIC"
}

type harness struct {
	c      *Collector
	out    chan protocol.Envelope
	ctx    context.Context
	cancel context.CancelFunc
}

func newHarness(bufferSize int) *harness {
	out := make(chan protocol.Envelope, bufferSize)
	ctx, cancel := context.WithCancel(context.Background())
	return &harness{
		c:      New("test-host", out),
		out:    out,
		ctx:    ctx,
		cancel: cancel,
	}
}

func TestCollector_Run(t *testing.T) {
	h := newHarness(10)
	defer h.cancel()

	var runCount int
	var mu sync.Mutex

	collectFn := func(ctx context.Context) ([]protocol.Metric, error) {
		mu.Lock()
		defer mu.Unlock()
		runCount++
		return []protocol.Metric{mockMetric{Value: runCount}}, nil
	}

	go h.c.Run(h.ctx, 50*time.Millisecond, collectFn)

	// Verify Baseline
	select {
	case env := <-h.out:
		if env.Hostname != "test-host" {
			t.Errorf("expected hostname 'test-host', got %s", env.Hostname)
		}
		if env.Type != "MOCK_METRIC" {
			t.Errorf("expected type 'MOCK_METRIC', got %s", env.Type)
		}
		if env.Timestamp.IsZero() {
			t.Error("expected valid timestamp, got zero")
		}

		// Verify payload
		if m, ok := env.Data.(mockMetric); !ok || m.Value != 1 {
			t.Errorf("expected data value 1, got %v", env.Data)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for baseline collection")
	}

	// Verify Ticker
	select {
	case env := <-h.out:
		if m, ok := env.Data.(mockMetric); !ok || m.Value != 2 {
			t.Errorf("expected data value 2 (ticker run), got %v", env.Data)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for ticked collection")
	}
}

func TestCollector_PanicRecovery(t *testing.T) {
	h := newHarness(10)
	defer h.cancel()

	var callCount int
	var mu sync.Mutex

	// Panic on first run, succeed on second to test runner survival
	flakyCollect := func(ctx context.Context) ([]protocol.Metric, error) {
		mu.Lock()
		defer mu.Unlock()
		callCount++

		if callCount == 1 {
			panic("something went wrong")
		}
		return []protocol.Metric{mockMetric{Value: 999}}, nil
	}

	go h.c.Run(h.ctx, 10*time.Millisecond, flakyCollect)

	time.Sleep(50 * time.Millisecond)

	select {
	case env := <-h.out:
		if m, ok := env.Data.(mockMetric); !ok || m.Value != 999 {
			t.Errorf("expected recovered value 999, got %v", env.Data)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("collector died after panic; expected it to recover and continue")
	}
}

func TestCollector_ErrorHandling(t *testing.T) {
	h := newHarness(5)
	defer h.cancel()

	errorCollect := func(ctx context.Context) ([]protocol.Metric, error) {
		return nil, errors.New("fail")
	}

	go h.c.Run(h.ctx, 10*time.Millisecond, errorCollect)

	select {
	case <-h.out:
		t.Fatal("Should not send envelope on error")
	case <-time.After(50 * time.Millisecond):
		// Success
	}
}

func TestCollector_ContextCancellation(t *testing.T) {
	// Unbuffered to prevent blocking forever
	h := newHarness(0)
	// Cancel immediately
	h.cancel()

	done := make(chan struct{})
	go func() {
		h.c.Run(h.ctx, time.Hour, func(ctx context.Context) ([]protocol.Metric, error) {
			return []protocol.Metric{mockMetric{}}, nil
		})
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Run() hung on cancelled context")
	}
}

func TestCollector_EmptyMetrics(t *testing.T) {
	h := newHarness(5)
	defer h.cancel()

	emptyCollect := func(ctx context.Context) ([]protocol.Metric, error) {
		return []protocol.Metric{}, nil
	}

	go h.c.Run(h.ctx, 10*time.Millisecond, emptyCollect)

	select {
	case <-h.out:
		t.Fatal("should not send envelope for empty metrics")
	case <-time.After(50 * time.Millisecond):
		// Success
	}
}

func TestCollector_MultipleMetrics(t *testing.T) {
	h := newHarness(10)
	defer h.cancel()

	batchCollect := func(ctx context.Context) ([]protocol.Metric, error) {
		return []protocol.Metric{
			mockMetric{Value: 1},
			mockMetric{Value: 2},
			mockMetric{Value: 3},
		}, nil
	}

	go h.c.Run(h.ctx, time.Hour, batchCollect)

	var received []int
	timeout := time.After(100 * time.Millisecond)
	for range 3 {
		select {
		case env := <-h.out:
			if m, ok := env.Data.(mockMetric); ok {
				received = append(received, m.Value)
			}
		case <-timeout:
			t.Fatalf("expected 3 metrics, got %d", len(received))
		}
	}

	if len(received) != 3 {
		t.Errorf("expected 3 metrics, got %d", len(received))
	}
}

func TestCollector_CancelDuringSend(t *testing.T) {
	// Unbuffered to block on send
	h := newHarness(0)

	slowCollect := func(ctx context.Context) ([]protocol.Metric, error) {
		return []protocol.Metric{mockMetric{Value: 1}}, nil
	}

	done := make(chan struct{})
	go func() {
		h.c.Run(h.ctx, time.Hour, slowCollect)
		close(done)
	}()

	// Give it time to attempt the send, then cancel
	time.Sleep(10 * time.Millisecond)
	h.cancel()

	select {
	case <-done:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Run() hung when context cancelled during blocked send")
	}
}

func TestCollector_NilMetricInSlice(t *testing.T) {
	h := newHarness(5)
	defer h.cancel()

	nilCollect := func(ctx context.Context) ([]protocol.Metric, error) {
		return []protocol.Metric{
			mockMetric{Value: 1}, nil, mockMetric{Value: 2},
		}, nil
	}

	go h.c.Run(h.ctx, time.Hour, nilCollect)

	expectedValues := []int{1, 2}

	for i, expected := range expectedValues {
		select {
		case env := <-h.out:
			if m, ok := env.Data.(mockMetric); !ok || m.Value != expected {
				t.Errorf("Message %d: expected value %d, got %v", i, expected, env.Data)
			}
		case <-time.After(50 * time.Millisecond):
			t.Fatalf("Timeout waiting for message %d", i)
		}
	}

	// Ensure nil didn't produce an envelope
	select {
	case env := <-h.out:
		t.Errorf("Received unexpected extra envelope: %+v", env)
	case <-time.After(50 * time.Millisecond):
	}
}

func BenchmarkCollector_Wrap(b *testing.B) {
	c := New("test-host", make(chan protocol.Envelope, 100))
	m := mockMetric{Value: 42}

	b.ResetTimer()
	for b.Loop() {
		_ = c.wrap(m)
	}
}
