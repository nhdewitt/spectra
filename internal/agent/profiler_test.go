//go:build spectraprof

package agent

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime/debug"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func TestEnvDuration(t *testing.T) {
	tests := []struct {
		name string
		set  string
		want time.Duration
	}{
		{"unset falls back", "", time.Minute},
		{"valid value is used", "30s", 30 * time.Second},
		{"unparseable falls back", "banana", time.Minute},
		{"zero falls back", "0s", time.Minute},
		{"negative falls back", "-5m", time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.set != "" {
				t.Setenv(envProfileInterval, tt.set)
			}
			if got := envDuration(envProfileInterval, time.Minute); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEnvInt(t *testing.T) {
	tests := []struct {
		name string
		set  string
		want int
	}{
		{"unset falls back", "", 30},
		{"valid value is used", "45", 45},
		{"unparseable falls back", "banana", 30},
		{"zero falls back", "0", 30},
		{"negative falls back", "-1", 30},
		{"above max falls back", "9999", 30},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.set != "" {
				t.Setenv(envProfileCPUSecs, tt.set)
			}
			if got := envInt(envProfileCPUSecs, 30, maxCPUSeconds); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// Both profiles must be readable by the pprof tooling, which means a non-empty
// gzip stream rather than whatever the buffer happened to contain.
func TestCaptureNamed(t *testing.T) {
	for _, name := range []string{"heap", "goroutine"} {
		t.Run(name, func(t *testing.T) {
			data, err := captureNamed(name)
			if err != nil {
				t.Fatalf("captureNamed(%q): %v", name, err)
			}
			if len(data) == 0 {
				t.Fatalf("captureNamed(%q) returned no bytes", name)
			}
			if data[0] != 0x1f || data[1] != 0x8b {
				t.Errorf("profile is not gzipped: % x", data[:2])
			}
		})
	}
}

func TestCaptureNamed_UnknownProfile(t *testing.T) {
	if _, err := captureNamed("no-such-profile"); err == nil {
		t.Error("expected an error for an unknown profile name")
	}
}

func TestCaptureCPU(t *testing.T) {
	data, err := captureCPU(t.Context(), 1)
	if err != nil {
		t.Fatalf("captureCPU: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("captureCPU returned no bytes")
	}
}

// A cancelled capture is a partial one. Returning it would understate CPU cost
// for that cycle, so the error matters more than the bytes.
func TestCaptureCPU_CancelledReturnsError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := captureCPU(ctx, 60); err == nil {
		t.Error("expected an error from a cancelled capture")
	}
}

// StartCPUProfile refuses a second concurrent profile, which is what happens
// when the -debug pprof server is serving /debug/pprof/profile on the same
// host. It must surface as an error, not a hang or a panic.
func TestCaptureCPU_SecondConcurrentFails(t *testing.T) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		if _, err := captureCPU(t.Context(), 1); err != nil {
			t.Errorf("first capture failed: %v", err)
		}
	}()

	// Give the first capture time to take the runtime's profiling slot.
	time.Sleep(100 * time.Millisecond)

	if _, err := captureCPU(t.Context(), 1); err == nil {
		t.Error("expected the second concurrent capture to fail")
	}

	<-done
}

func TestSend_PostsProfileAndMetadata(t *testing.T) {
	var got profileUpload
	var contentType string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentType = r.Header.Get("Content-Type")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
			return
		}
		if err := json.Unmarshal(body, &got); err != nil {
			t.Errorf("unmarshal body: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	a := newTestAgentWithLogger()
	up := profileUpload{
		AgentID:  a.Identity.ID,
		Hostname: "test-host",
		OS:       "linux",
		Arch:     "arm64",
		CPUModel: "Test CPU",
		CPUCores: 4,
		Kind:     "cpu",
		Profile:  []byte{0x1f, 0x8b, 0x00},
	}

	if err := a.send(t.Context(), srv.Client(), srv.URL, up); err != nil {
		t.Fatalf("send: %v", err)
	}

	if contentType != "application/json" {
		t.Errorf("Content-Type = %q", contentType)
	}
	if got.AgentID != a.Identity.ID {
		t.Errorf("agent_id = %q, want %q", got.AgentID, a.Identity.ID)
	}
	if got.CPUModel != "Test CPU" || got.CPUCores != 4 {
		t.Errorf("host facts lost: model=%q cores=%d", got.CPUModel, got.CPUCores)
	}
	if len(got.Profile) != 3 {
		t.Errorf("profile bytes = %d, want 3", len(got.Profile))
	}
}

// The scalars a pprof profile cannot carry. GOMEMLIMIT in particular differs
// per host by design, so without it two agents' heap profiles are not
// comparable.
func TestCollectRuntimeSnapshot(t *testing.T) {
	snap := collectRuntimeSnapshot()

	if snap.GOMAXPROCS < 1 {
		t.Errorf("GOMAXPROCS = %d, want at least 1", snap.GOMAXPROCS)
	}
	if snap.Goroutines == 0 {
		t.Error("goroutines = 0; the test itself is running")
	}
	if snap.HeapObjectsBytes == 0 {
		t.Error("heap objects = 0")
	}
	if snap.TotalBytes == 0 {
		t.Error("total bytes = 0")
	}
	if snap.GOGC == 0 {
		t.Error("GOGC = 0; expected the runtime's percent setting")
	}
	if snap.TotalCPUSec <= 0 {
		t.Errorf("total cpu seconds = %v, want positive", snap.TotalCPUSec)
	}
}

// debug.SetMemoryLimit applies a ceiling; the snapshot must report it rather
// than the math/MaxInt64 sentinel that means "no limit".
func TestCollectRuntimeSnapshot_ReportsMemoryLimit(t *testing.T) {
	const limit = 512 << 20

	prev := debug.SetMemoryLimit(limit)
	t.Cleanup(func() { debug.SetMemoryLimit(prev) })

	if got := collectRuntimeSnapshot().GOMEMLIMIT; got != limit {
		t.Errorf("GOMEMLIMIT = %d, want %d", got, limit)
	}
}

func TestSend_CarriesRuntimeSnapshot(t *testing.T) {
	var got profileUpload

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
			return
		}
		if err := json.Unmarshal(body, &got); err != nil {
			t.Errorf("unmarshal body: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	a := newTestAgentWithLogger()
	a.push(t.Context(), srv.Client(), srv.URL, protocol.HostInfo{Hostname: "test-host"}, "heap", 0, []byte{0x1f, 0x8b})

	if got.Runtime.GOMAXPROCS < 1 {
		t.Errorf("snapshot did not survive the round trip: %+v", got.Runtime)
	}
	if got.Runtime.Goroutines == 0 {
		t.Error("goroutines did not survive the round trip")
	}
}

func TestSend_NonSuccessIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	a := newTestAgentWithLogger()
	err := a.send(t.Context(), srv.Client(), srv.URL, profileUpload{Kind: "heap"})
	if err == nil {
		t.Error("expected an error for a 500 response")
	}
}

// One failing profile must not end the loop; a run that dies silently on the
// first hiccup is worse than a gap in the data.
func TestCaptureCycle_SurvivesAggregatorFailure(t *testing.T) {
	var posts atomic.Int64

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		posts.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	a := newTestAgentWithLogger()
	a.captureCycle(t.Context(), srv.Client(), srv.URL, protocol.HostInfo{Hostname: "test-host"}, 1)

	// cpu, heap and goroutine all attempted despite every upload failing.
	if got := posts.Load(); got != 3 {
		t.Errorf("aggregator saw %d posts, want 3", got)
	}
}
