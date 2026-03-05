//go:build darwin

package collector

import (
	"testing"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func TestParseLaunchctlList_Basic(t *testing.T) {
	data := []byte("PID\tStatus\tLabel\n" +
		"-\t0\tcom.apple.some.agent\n" +
		"501\t0\tcom.apple.WindowServer\n" +
		"-\t78\tcom.apple.crashed\n")

	metrics, err := parseLaunchctlList(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plm, ok := metrics[0].(protocol.ServiceListMetric)
	if !ok {
		t.Fatalf("expected ServiceListMetric, got %T", metrics[0])
	}
	if len(plm.Services) != 3 {
		t.Fatalf("got %d services, want 3", len(plm.Services))
	}

	tests := []struct {
		name, status, substatus string
	}{
		{"com.apple.some.agent", "stopped", "exited"},
		{"com.apple.WindowServer", "running", "running"},
		{"com.apple.crashed", "stopped", "failed"},
	}

	for i, tt := range tests {
		svc := plm.Services[i]
		if svc.Name != tt.name {
			t.Errorf("[%d] Name = %q, want %q", i, svc.Name, tt.name)
		}
		if svc.LoadState != "loaded" {
			t.Errorf("[%d] LoadState = %q, want loaded", i, svc.LoadState)
		}
		if svc.Status != tt.status {
			t.Errorf("[%d] Status = %q, want %q", i, svc.Status, tt.status)
		}
		if svc.SubStatus != tt.substatus {
			t.Errorf("[%d] SubStatus = %q, want %q", i, svc.SubStatus, tt.substatus)
		}
	}
}
