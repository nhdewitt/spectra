package sender

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func randomMetric() protocol.Metric {
	switch rand.Intn(15) {
	case 0:
		return protocol.CPUMetric{
			Usage:     rand.Float64() * 100,
			CoreUsage: []float64{rand.Float64() * 100, rand.Float64() * 100, rand.Float64() * 100, rand.Float64() * 100},
			LoadAvg1:  rand.Float64() * 4,
			LoadAvg5:  rand.Float64() * 4,
			LoadAvg15: rand.Float64() * 4,
		}
	case 1:
		return protocol.MemoryMetric{
			Total:     16 * 1024 * 1024 * 1024,
			Used:      rand.Uint64() % (16 * 1024 * 1024 * 1024),
			Available: rand.Uint64() % (16 * 1024 * 1024 * 1024),
			UsedPct:   rand.Float64() * 100,
			SwapTotal: 8 * 1024 * 1024 * 1024,
			SwapUsed:  rand.Uint64() % (8 * 1024 * 1024 * 1024),
			SwapPct:   rand.Float64() * 100,
		}
	case 2:
		return protocol.DiskMetric{
			Device:      "/dev/sda1",
			Mountpoint:  "/",
			Filesystem:  "ext4",
			Type:        "ssd",
			Total:       500 * 1024 * 1024 * 1024,
			Used:        rand.Uint64() % (500 * 1024 * 1024 * 1024),
			Available:   rand.Uint64() % (500 * 1024 * 1024 * 1024),
			UsedPct:     rand.Float64() * 100,
			InodesTotal: 1000000,
			InodesUsed:  rand.Uint64() % 1000000,
			InodesPct:   rand.Float64() * 100,
		}
	case 3:
		return protocol.DiskIOMetric{
			Device:     "sda",
			ReadBytes:  rand.Uint64(),
			WriteBytes: rand.Uint64(),
			ReadOps:    rand.Uint64(),
			WriteOps:   rand.Uint64(),
			ReadTime:   rand.Uint64() % 10000,
			WriteTime:  rand.Uint64() % 10000,
			InProgress: rand.Uint64() % 100,
		}
	case 4:
		return protocol.NetworkMetric{
			Interface: "eth0",
			MAC:       "00:11:22:33:44:55",
			MTU:       1500,
			Speed:     1000,
			RxBytes:   rand.Uint64(),
			RxPackets: rand.Uint64(),
			RxErrors:  rand.Uint64() % 100,
			RxDrops:   rand.Uint64() % 100,
			TxBytes:   rand.Uint64(),
			TxPackets: rand.Uint64(),
			TxErrors:  rand.Uint64() % 100,
			TxDrops:   rand.Uint64() % 100,
		}
	case 5:
		return protocol.TemperatureMetric{
			Sensor: "coretemp-isa-0000",
			Temp:   40 + rand.Float64()*40,
			Max:    100,
		}
	case 6:
		return protocol.SystemMetric{
			Uptime:    rand.Uint64() % (86400 * 365),
			Processes: rand.Intn(500),
			Users:     rand.Intn(10),
			BootTime:  uint64(time.Now().Add(-time.Hour * 24 * 30).Unix()),
		}
	case 7:
		return protocol.ProcessMetric{
			Pid:          rand.Intn(65535),
			Name:         "some-process-name",
			CPUPercent:   rand.Float64() * 100,
			MemPercent:   rand.Float64() * 100,
			MemRSS:       rand.Uint64() % (4 * 1024 * 1024 * 1024),
			Status:       protocol.ProcRunning,
			ThreadsTotal: uint32(rand.Intn(100)),
		}
	case 8:
		return protocol.WiFiMetric{
			Interface:   "wlan0",
			SSID:        "MyNetwork",
			SignalLevel: -50 - rand.Intn(40),
			LinkQuality: rand.Intn(100),
			Frequency:   5.0 + rand.Float64(),
			BitRate:     rand.Float64() * 1000,
		}
	case 9:
		return protocol.ThrottleMetric{
			Undervoltage:  rand.Intn(2) == 1,
			Throttled:     rand.Intn(2) == 1,
			SoftTempLimit: rand.Intn(2) == 1,
		}
	case 10:
		return protocol.ClockMetric{
			ArmFreq:  uint64(1500000000 + rand.Intn(500000000)),
			CoreFreq: uint64(500000000 + rand.Intn(100000000)),
			GPUFreq:  uint64(400000000 + rand.Intn(100000000)),
		}
	case 11:
		return protocol.VoltageMetric{
			Core:   1.2 + rand.Float64()*0.2,
			SDRamC: 1.1 + rand.Float64()*0.1,
			SDRamI: 1.1 + rand.Float64()*0.1,
			SDRamP: 1.1 + rand.Float64()*0.1,
		}
	case 12:
		return protocol.GPUMetric{
			MemoryTotal: 8 * 1024 * 1024 * 1024,
			MemoryUsed:  rand.Uint64() % (8 * 1024 * 1024 * 1024),
		}
	case 13:
		return protocol.ContainerMetric{
			ID:            "abc123def456",
			Name:          "nginx-proxy",
			Image:         "nginx:latest",
			State:         "running",
			Source:        "docker",
			Kind:          "container",
			CPUPercent:    rand.Float64() * 100,
			CPULimitCores: 4,
			MemoryBytes:   rand.Uint64() % (4 * 1024 * 1024 * 1024),
			MemoryLimit:   4 * 1024 * 1024 * 1024,
			NetRxBytes:    rand.Uint64(),
			NetTxBytes:    rand.Uint64(),
		}
	case 14:
		return protocol.ServiceMetric{
			Name:        "nginx.service",
			Status:      "active",
			SubStatus:   "running",
			LoadState:   "loaded",
			Description: "A high performance web server",
		}
	default:
		return protocol.CPUMetric{Usage: rand.Float64() * 100}
	}
}

func randomEnvelope() protocol.Envelope {
	metric := randomMetric()
	return protocol.Envelope{
		Type:      metric.MetricType(),
		Timestamp: time.Now(),
		Hostname:  "test-host",
		Data:      metric,
	}
}

func makeMixedBatch(n int) []protocol.Envelope {
	batch := make([]protocol.Envelope, n)
	for i := range batch {
		batch[i] = randomEnvelope()
	}
	return batch
}

func TestNew(t *testing.T) {
	ch := make(chan protocol.Envelope)
	s := New("http://localhost:8080/metrics", ch)

	if s.endpoint != "http://localhost:8080/metrics" {
		t.Errorf("endpoint: got %s, want http://localhost:8080/metrics", s.endpoint)
	}
	if s.in != ch {
		t.Error("input channel not set correctly")
	}
	if s.client == nil {
		t.Error("client should not be nil")
	}
	if s.maxBatch != 50 {
		t.Errorf("maxBatch: got %d, want 50", s.maxBatch)
	}
	if s.flush != 5*time.Second {
		t.Errorf("flush: got %v, want 5s", s.flush)
	}
	if cap(s.batch) != 50 {
		t.Errorf("batch capacity: got %d, want 50", cap(s.batch))
	}
}

func TestSender_SendBatch_Empty(t *testing.T) {
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ch := make(chan protocol.Envelope)
	s := New(server.URL, ch)

	s.sendBatch()

	if atomic.LoadInt32(&requestCount) != 0 {
		t.Error("sendBatch should not make request when batch is empty")
	}
}

func TestSender_SendBatch_Success(t *testing.T) {
	var receivedBatch []protocol.Envelope
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method: got %s, want POST", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("content-type: got %s, want application/json", r.Header.Get("Content-Type"))
		}

		body, _ := io.ReadAll(r.Body)

		var prettyJSON bytes.Buffer
		if err := json.Indent(&prettyJSON, body, "", " "); err != nil {
			t.Logf("Raw body: %s", body)
		} else {
			t.Logf("Request JSON:\n%s", prettyJSON.String())
		}
		mu.Lock()
		json.Unmarshal(body, &receivedBatch)
		mu.Unlock()

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ch := make(chan protocol.Envelope)
	s := New(server.URL, ch)

	s.batch = append(s.batch, randomEnvelope())
	s.batch = append(s.batch, randomEnvelope())

	s.sendBatch()

	mu.Lock()
	defer mu.Unlock()

	if len(receivedBatch) != 2 {
		t.Errorf("received %d envelopes, want 2", len(receivedBatch))
	}
	if len(s.batch) != 0 {
		t.Errorf("batch should be empty after send, got %d items", len(s.batch))
	}
}

func TestSender_SendBatch_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)

		body, _ := io.ReadAll(r.Body)

		var prettyJSON bytes.Buffer
		if err := json.Indent(&prettyJSON, body, "", " "); err != nil {
			t.Logf("Raw body: %s", body)
		} else {
			t.Logf("Request JSON:\n%s", prettyJSON.String())
		}
	}))
	defer server.Close()

	ch := make(chan protocol.Envelope)
	s := New(server.URL, ch)

	s.batch = append(s.batch, randomEnvelope())
	s.sendBatch()

	if len(s.batch) != 0 {
		t.Errorf("batch should be cleared even on error, got %d items", len(s.batch))
	}
}

func TestSender_SendBatch_ConnectionError(t *testing.T) {
	ch := make(chan protocol.Envelope)
	s := New("http://localhost:59999", ch)

	s.batch = append(s.batch, randomEnvelope())
	s.sendBatch()

	if len(s.batch) != 0 {
		t.Errorf("batch should be cleared even on connection error, got %d items", len(s.batch))
	}
}

func TestSender_Run_MaxBatchTrigger(t *testing.T) {
	var batchSizes []int
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var batch []protocol.Envelope
		json.Unmarshal(body, &batch)

		mu.Lock()
		batchSizes = append(batchSizes, len(batch))
		mu.Unlock()

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ch := make(chan protocol.Envelope, 100)
	s := New(server.URL, ch)
	s.maxBatch = 10
	s.flush = 1 * time.Hour

	ctx, cancel := context.WithCancel(context.Background())

	go s.Run(ctx)

	// Send 25 envelopes - should trigger 2 batches of 10
	for range 25 {
		ch <- randomEnvelope()
	}

	time.Sleep(100 * time.Millisecond)
	cancel()
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	fullBatches := 0
	for _, size := range batchSizes {
		if size == 10 {
			fullBatches++
		}
	}

	if fullBatches < 2 {
		t.Errorf("expected at least 2 full batches of 10, got sizes: %v", batchSizes)
	}
}

func TestSender_Run_FlushTrigger(t *testing.T) {
	var batchCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&batchCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ch := make(chan protocol.Envelope, 10)
	s := New(server.URL, ch)
	s.maxBatch = 100
	s.flush = 50 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())

	go s.Run(ctx)

	for range 5 {
		ch <- randomEnvelope()
	}

	time.Sleep(150 * time.Millisecond)
	cancel()
	time.Sleep(50 * time.Millisecond)

	if atomic.LoadInt32(&batchCount) < 1 {
		t.Error("expected at least 1 batch from flush timer")
	}
}

func TestSender_Run_ContextCancel(t *testing.T) {
	var finalBatchSize int
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var batch []protocol.Envelope
		json.Unmarshal(body, &batch)

		mu.Lock()
		finalBatchSize = len(batch)
		mu.Unlock()

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ch := make(chan protocol.Envelope, 10)
	s := New(server.URL, ch)
	s.maxBatch = 100
	s.flush = 1 * time.Hour

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		s.Run(ctx)
		close(done)
	}()

	for range 3 {
		ch <- randomEnvelope()
	}

	time.Sleep(50 * time.Millisecond)

	cancel() // Should flush remaining

	select {
	case <-done:
		// Good
	case <-time.After(1 * time.Second):
		t.Fatal("Run did not exit after context cancel")
	}

	mu.Lock()
	defer mu.Unlock()

	if finalBatchSize != 3 {
		t.Errorf("expected final batch of 3 on shutdown, got %d", finalBatchSize)
	}
}

func TestSender_Run_EmptyOnCancel(t *testing.T) {
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ch := make(chan protocol.Envelope)
	s := New(server.URL, ch)
	s.flush = 1 * time.Hour

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		s.Run(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
		// Good
	case <-time.After(1 * time.Second):
		t.Fatal("Run did not exit after context cancel")
	}

	if atomic.LoadInt32(&requestCount) != 0 {
		t.Errorf("expected no requests for empty batch, got %d", requestCount)
	}
}

func TestSender_BatchClearing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ch := make(chan protocol.Envelope)
	s := New(server.URL, ch)

	for range 10 {
		s.batch = append(s.batch, randomEnvelope())
	}

	originalCap := cap(s.batch)
	s.sendBatch()

	if cap(s.batch) != originalCap {
		t.Errorf("batch capacity changed: got %d, want %d", cap(s.batch), originalCap)
	}

	if len(s.batch) != 0 {
		t.Errorf("batch length should be 0, got %d", len(s.batch))
	}
}

func BenchmarkMarshalMixedBatch_10(b *testing.B) {
	batch := makeMixedBatch(10)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = json.Marshal(batch)
	}
}

func BenchmarkMarshalMixedBatch_50(b *testing.B) {
	batch := makeMixedBatch(50)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = json.Marshal(batch)
	}
}

func BenchmarkMarshalMixedBatch_100(b *testing.B) {
	batch := makeMixedBatch(100)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = json.Marshal(batch)
	}
}

func BenchmarkMarshalMetric_CPU(b *testing.B) {
	e := protocol.Envelope{
		Type:      "cpu",
		Timestamp: time.Now(),
		Hostname:  "test",
		Data: protocol.CPUMetric{
			Usage:     75.5,
			CoreUsage: []float64{80, 70, 85, 65, 90, 60, 75, 80},
			LoadAvg1:  2.5,
			LoadAvg5:  2.0,
			LoadAvg15: 1.5,
		},
	}

	b.ReportAllocs()
	for b.Loop() {
		_, _ = json.Marshal(e)
	}
}

func BenchmarkMarshalMetric_ProcessList(b *testing.B) {
	procs := make([]protocol.ProcessMetric, 100)
	for i := range procs {
		procs[i] = protocol.ProcessMetric{
			Pid:          i + 1,
			Name:         "process-name",
			CPUPercent:   rand.Float64() * 100,
			MemPercent:   rand.Float64() * 100,
			MemRSS:       rand.Uint64() % (4 * 1024 * 1024 * 1024),
			Status:       protocol.ProcRunning,
			ThreadsTotal: uint32(rand.Intn(100)),
		}
	}

	e := protocol.Envelope{
		Type:      "process_list",
		Timestamp: time.Now(),
		Hostname:  "test",
		Data:      protocol.ProcessListMetric{Processes: procs},
	}

	b.ReportAllocs()
	for b.Loop() {
		_, _ = json.Marshal(e)
	}
}

func BenchmarkMarshalMetric_ContainerList(b *testing.B) {
	containers := make([]protocol.ContainerMetric, 20)
	for i := range containers {
		containers[i] = protocol.ContainerMetric{
			ID:            "abc123def456",
			Name:          "container-name",
			Image:         "image:latest",
			State:         "running",
			Source:        "docker",
			Kind:          "container",
			CPUPercent:    rand.Float64() * 100,
			CPULimitCores: 4,
			MemoryBytes:   rand.Uint64() % (4 * 1024 * 1024 * 1024),
			MemoryLimit:   4 * 1024 * 1024 * 1024,
			NetRxBytes:    rand.Uint64(),
			NetTxBytes:    rand.Uint64(),
		}
	}

	e := protocol.Envelope{
		Type:      "container_list",
		Timestamp: time.Now(),
		Hostname:  "test",
		Data:      protocol.ContainerListMetric{Containers: containers},
	}

	b.ReportAllocs()
	for b.Loop() {
		_, _ = json.Marshal(e)
	}
}

func BenchmarkMarshalMetric_ServiceList(b *testing.B) {
	services := make([]protocol.ServiceMetric, 50)
	for i := range services {
		services[i] = protocol.ServiceMetric{
			Name:        "service-name.service",
			Status:      "active",
			SubStatus:   "running",
			LoadState:   "loaded",
			Description: "Some service description here",
		}
	}

	e := protocol.Envelope{
		Type:      "service_list",
		Timestamp: time.Now(),
		Hostname:  "test",
		Data:      protocol.ServiceListMetric{Services: services},
	}

	b.ReportAllocs()
	for b.Loop() {
		_, _ = json.Marshal(e)
	}
}

func BenchmarkSendBatch_Small(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ch := make(chan protocol.Envelope)
	s := New(server.URL, ch)

	for range 10 {
		s.batch = append(s.batch, randomEnvelope())
	}
	batchCopy := make([]protocol.Envelope, len(s.batch))
	copy(batchCopy, s.batch)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		s.batch = append(s.batch[:0], batchCopy...)
		s.sendBatch()
	}
}

func BenchmarkSendBatch_Large(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ch := make(chan protocol.Envelope)
	s := New(server.URL, ch)

	for range 50 {
		s.batch = append(s.batch, randomEnvelope())
	}
	batchCopy := make([]protocol.Envelope, len(s.batch))
	copy(batchCopy, s.batch)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		s.batch = append(s.batch[:0], batchCopy...)
		s.sendBatch()
	}
}
