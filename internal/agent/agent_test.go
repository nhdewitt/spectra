package agent

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func TestNew(t *testing.T) {
	cfg := Config{
		BaseURL:      "http://localhost:8080",
		Hostname:     "test-agent",
		MetricsPath:  "/api/v1/metrics",
		CommandPath:  "/api/v1/agent/command",
		PollInterval: 5 * time.Second,
	}

	a := New(cfg)

	if a.Config.BaseURL != cfg.BaseURL {
		t.Errorf("BaseURL: got %s, want %s", a.Config.BaseURL, cfg.BaseURL)
	}
	if a.Config.Hostname != cfg.Hostname {
		t.Errorf("Hostname: got %s, want %s", a.Config.Hostname, cfg.Hostname)
	}
	if a.Config.PollInterval != cfg.PollInterval {
		t.Errorf("PollInterval: got %v, want %v", a.Config.PollInterval, cfg.PollInterval)
	}
	if a.Client == nil {
		t.Error("Client should not be nil")
	}
	if a.Client.Timeout != 45*time.Second {
		t.Errorf("Client.Timeout: got %v, want 45s", a.Client.Timeout)
	}
	if a.DriveCache == nil {
		t.Error("DriveCache should not be nil")
	}
	if a.metricsCh == nil {
		t.Error("metricsCh should not be nil")
	}
	if cap(a.metricsCh) != 500 {
		t.Errorf("metricsCh capacity: got %d, want 500", cap(a.metricsCh))
	}
	if a.batch == nil {
		t.Error("batch should not be nil")
	}
	if cap(a.batch) != 50 {
		t.Errorf("batch capacity: got %d, want 50", cap(a.batch))
	}
	if a.ctx == nil {
		t.Error("ctx should not be nil")
	}
	if a.cancel == nil {
		t.Error("cancel should not be nil")
	}
	if a.gzipW == nil {
		t.Error("gzipW should not be nil")
	}
	if a.commonHeaders == nil {
		t.Error("commonHeaders should not be nil")
	}
	if a.RetryConfig.MaxAttempts != 3 {
		t.Errorf("RetryConfig.MaxAttempts: got %d, want 3", a.RetryConfig.MaxAttempts)
	}
	if a.RetryConfig.InitialDelay != 1*time.Second {
		t.Errorf("RetryConfig.InitialDelay: got %v, want 1s", a.RetryConfig.InitialDelay)
	}
	if a.RetryConfig.MaxDelay != 30*time.Second {
		t.Errorf("RetryConfig.MaxDelay: got %v, want 30s", a.RetryConfig.MaxDelay)
	}
	if a.RetryConfig.Multiplier != 2.0 {
		t.Errorf("RetryConfig.Multiplier: got %f, want 2.0", a.RetryConfig.Multiplier)
	}
}

func TestNew_CommonHeaders(t *testing.T) {
	a := New(Config{})

	expected := map[string]string{
		"Content-Type":     "application/json",
		"Content-Encoding": "gzip",
		"User-Agent":       "Spectra-Agent/1.0",
	}

	for k, want := range expected {
		got, ok := a.commonHeaders[k]
		if !ok {
			t.Errorf("missing header %s", k)
			continue
		}
		if got != want {
			t.Errorf("header %s: got %s, want %s", k, got, want)
		}
	}
}

func TestAgent_SetHeaders(t *testing.T) {
	a := New(Config{})

	req, _ := http.NewRequest(http.MethodPost, "http://example.com", nil)
	a.setHeaders(req)

	if req.Header.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type: got %s, want application/json", req.Header.Get("Content-Type"))
	}
	if req.Header.Get("Content-Encoding") != "gzip" {
		t.Errorf("Content-Encoding: got %s, want gzip", req.Header.Get("Content-Encoding"))
	}
	if req.Header.Get("User-Agent") != "Spectra-Agent/1.0" {
		t.Errorf("User-Agent: got %s, want Spectra-Agent/1.0", req.Header.Get("User-Agent"))
	}
}

func TestAgent_Shutdown(t *testing.T) {
	a := New(Config{})

	done := make(chan struct{})
	go func() {
		a.Shutdown()
		close(done)
	}()

	select {
	case <-done:
		// Good - shutdown completed
	case <-time.After(1 * time.Second):
		t.Error("Shutdown did not complete in time")
	}

	select {
	case <-a.ctx.Done():
		// Good
	default:
		t.Error("context should be cancelled after shutdown")
	}
}

func TestAgent_Shutdown_WithWaitGroup(t *testing.T) {
	a := New(Config{})

	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		<-a.ctx.Done()
	}()

	done := make(chan struct{})
	go func() {
		a.Shutdown()
		close(done)
	}()

	select {
	case <-done:
		// Good
	case <-time.After(1 * time.Second):
		t.Error("Shutdown did not complete in time")
	}
}

func TestAgent_MetricsChannel(t *testing.T) {
	a := New(Config{})

	for i := range 100 {
		select {
		case a.metricsCh <- protocol.Envelope{Type: "test"}:
			// Good
		default:
			t.Fatalf("channel blocked at %d sends", i)
		}
	}

	for i := range 100 {
		select {
		case env := <-a.metricsCh:
			if env.Type != "test" {
				t.Errorf("envelope type: got %s, want test", env.Type)
			}
		default:
			t.Fatalf("channel empty at %d receives", i)
		}
	}
}

func TestAgent_ContextCancellation(t *testing.T) {
	a := New(Config{})

	select {
	case <-a.ctx.Done():
		t.Error("context should not be done before cancel")
	default:
		// Good
	}

	a.cancel()

	select {
	case <-a.ctx.Done():
		// Good
	default:
		t.Error("context should be done after cancel")
	}

	if a.ctx.Err() != context.Canceled {
		t.Errorf("ctx.Err(): got %v, want context.Canceled", a.ctx.Err())
	}
}

func TestAgent_MultipleNew(t *testing.T) {
	a1 := New(Config{Hostname: "agent-1"})
	a2 := New(Config{Hostname: "agent-2"})

	if a1.Config.Hostname == a2.Config.Hostname {
		t.Error("agents should have different hostnames")
	}

	a1.cancel()

	select {
	case <-a1.ctx.Done():
		// Good
	default:
		t.Error("a1 context should be done")
	}

	select {
	case <-a2.ctx.Done():
		t.Error("a2 context should not be done")
	default:
		// Good
	}
}

func TestConfig_Defaults(t *testing.T) {
	cfg := Config{}

	if cfg.BaseURL != "" {
		t.Errorf("default BaseURL should be empty, got %s", cfg.BaseURL)
	}
	if cfg.Hostname != "" {
		t.Errorf("default Hostname should be empty, got %s", cfg.Hostname)
	}
	if cfg.PollInterval != 0 {
		t.Errorf("default PollInterval should be 0, got %v", cfg.PollInterval)
	}
}

func TestAgent_GzipBufferConcurrency(t *testing.T) {
	a := New(Config{})

	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			a.gzipMu.Lock()
			a.gzipBuf.Reset()
			a.gzipBuf.WriteString("test data")
			_ = a.gzipBuf.String()
			a.gzipMu.Unlock()
		}()
	}

	wg.Wait()
}

func TestDefaultRetryConfig(t *testing.T) {
	rc := DefaultRetryConfig()

	if rc.MaxAttempts != 3 {
		t.Errorf("MaxAttempts: got %d, want 3", rc.MaxAttempts)
	}
	if rc.InitialDelay != 1*time.Second {
		t.Errorf("InitialDelay: got %v, want 1s", rc.InitialDelay)
	}
	if rc.MaxDelay != 30*time.Second {
		t.Errorf("MaxDelay: got %v, want 30s", rc.MaxDelay)
	}
	if rc.Multiplier != 2.0 {
		t.Errorf("Multiplier: got %f, want 2.0", rc.Multiplier)
	}
}

func TestRetryConfig_Delay_NegativeAttempt(t *testing.T) {
	rc := RetryConfig{
		InitialDelay: 1 * time.Second,
		MaxDelay:     10 * time.Second,
		Multiplier:   2.0,
	}

	got := rc.Delay(-1)
	if got != rc.InitialDelay {
		t.Errorf("Delay(-1) = %v, want %v", got, rc.InitialDelay)
	}
}

func BenchmarkNew(b *testing.B) {
	cfg := Config{
		BaseURL:      "http://localhost:8080",
		Hostname:     "bench-agent",
		MetricsPath:  "/api/v1/metrics",
		CommandPath:  "/api/v1/agent/command",
		PollInterval: 5 * time.Second,
	}

	b.ReportAllocs()
	for b.Loop() {
		_ = New(cfg)
	}
}

func BenchmarkAgent_SetHeaders(b *testing.B) {
	a := New(Config{})
	req, _ := http.NewRequest(http.MethodPost, "http://example.com", nil)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		a.setHeaders(req)
	}
}

func BenchmarkAgent_MetricsChannel_Send(b *testing.B) {
	a := New(Config{})

	env := protocol.Envelope{Type: "cpu", Hostname: "test"}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		for {
			select {
			case <-a.metricsCh:
			case <-ctx.Done():
				return
			}
		}
	}()

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		a.metricsCh <- env
	}

	cancel()
}

func BenchmarkAgent_MetricsChannel_SendReceive(b *testing.B) {
	a := New(Config{})

	env := protocol.Envelope{Type: "cpu", Hostname: "test"}

	b.ReportAllocs()
	for b.Loop() {
		a.metricsCh <- env
		<-a.metricsCh
	}
}

func BenchmarkAgent_GzipBuffer_Lock(b *testing.B) {
	a := New(Config{})

	b.ReportAllocs()
	for b.Loop() {
		a.gzipMu.Lock()
		_ = a.gzipBuf.Len()
		a.gzipMu.Unlock()
	}
}

func BenchmarkAgent_GzipBuffer_WriteReset(b *testing.B) {
	a := New(Config{})
	data := []byte(`{"type":"cpu","hostname":"test","data":{"usage":50.5}}`)

	b.ReportAllocs()
	for b.Loop() {
		a.gzipMu.Lock()
		a.gzipBuf.Reset()
		a.gzipBuf.Write(data)
		a.gzipMu.Unlock()
	}
}

func BenchmarkConfig_Copy(b *testing.B) {
	cfg := Config{
		BaseURL:      "http://localhost:8080",
		Hostname:     "bench-agent",
		MetricsPath:  "/api/v1/metrics",
		CommandPath:  "/api/v1/agent/command",
		PollInterval: 5 * time.Second,
	}

	b.ReportAllocs()
	for b.Loop() {
		_ = cfg
	}
}

func BenchmarkRetryConfig_Delay(b *testing.B) {
	rc := DefaultRetryConfig()

	b.ReportAllocs()
	for b.Loop() {
		rc.Delay(0)
		rc.Delay(1)
		rc.Delay(2)
		rc.Delay(5)
	}
}
