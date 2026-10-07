package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func sampleMetrics() []protocol.Metric {
	return []protocol.Metric{
		protocol.DiskMetric{Mountpoint: "/", Filesystem: "ext4"},
		protocol.DiskMetric{Mountpoint: "/mnt/nas", Filesystem: "nfs"},
		protocol.NetworkMetric{Interface: "eth0"},
		protocol.NetworkMetric{Interface: "docker0"},
		protocol.CPUMetric{},
	}
}

func staticCollect(metrics []protocol.Metric, err error) func(context.Context) ([]protocol.Metric, error) {
	return func(context.Context) ([]protocol.Metric, error) {
		return metrics, err
	}
}

func TestIgnoreLists_Drops(t *testing.T) {
	ig := newIgnoreLists([]string{"nfs"}, []string{"docker0"})

	tests := []struct {
		name string
		m    protocol.Metric
		want bool
	}{
		{"ignored filesystem", protocol.DiskMetric{Filesystem: "nfs"}, true},
		{"kept filesystem", protocol.DiskMetric{Filesystem: "ext4"}, false},
		{"ignored interface", protocol.NetworkMetric{Interface: "docker0"}, true},
		{"kept interface", protocol.NetworkMetric{Interface: "eth0"}, false},
		{"filesystem name does not match an interface", protocol.NetworkMetric{Interface: "nfs"}, false},
		{"other metric types pass", protocol.CPUMetric{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ig.drops(tt.m); got != tt.want {
				t.Errorf("drops() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIgnoreLists_Equal(t *testing.T) {
	a := newIgnoreLists([]string{"nfs", "cifs"}, []string{"lo"})
	if !a.equal(newIgnoreLists([]string{"cifs", "nfs"}, []string{"lo"})) {
		t.Error("lists with the same entries in a different order should be equal")
	}
	if a.equal(newIgnoreLists([]string{"nfs"}, []string{"lo"})) {
		t.Error("lists with different filesystems should not be equal")
	}
	if a.equal(newIgnoreLists([]string{"nfs", "cifs"}, nil)) {
		t.Error("lists with different interfaces should not be equal")
	}
}

func TestWithIgnoreFilter_NoListsPassesEverything(t *testing.T) {
	a := newTestAgentWithLogger()

	got, err := a.withIgnoreFilter(staticCollect(sampleMetrics(), nil))(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 5 {
		t.Errorf("got %d metrics, want 5", len(got))
	}
}

func TestWithIgnoreFilter_DropsIgnored(t *testing.T) {
	a := newTestAgentWithLogger()
	a.ignore.Store(newIgnoreLists([]string{"nfs"}, []string{"docker0"}))

	got, err := a.withIgnoreFilter(staticCollect(sampleMetrics(), nil))(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d metrics, want 3: %+v", len(got), got)
	}
	for _, m := range got {
		switch v := m.(type) {
		case protocol.DiskMetric:
			if v.Filesystem == "nfs" {
				t.Error("nfs disk metric was not dropped")
			}
		case protocol.NetworkMetric:
			if v.Interface == "docker0" {
				t.Error("docker0 network metric was not dropped")
			}
		}
	}
}

func TestWithIgnoreFilter_ReadsCurrentLists(t *testing.T) {
	a := newTestAgentWithLogger()
	fn := a.withIgnoreFilter(func(context.Context) ([]protocol.Metric, error) {
		return sampleMetrics(), nil
	})

	a.ignore.Store(newIgnoreLists([]string{"nfs"}, nil))
	first, _ := fn(context.Background())

	a.ignore.Store(newIgnoreLists(nil, nil))
	second, _ := fn(context.Background())

	if len(first) != 4 || len(second) != 5 {
		t.Errorf("got %d then %d metrics, want 4 then 5", len(first), len(second))
	}
}

func TestWithIgnoreFilter_PassesErrorThrough(t *testing.T) {
	a := newTestAgentWithLogger()
	a.ignore.Store(newIgnoreLists([]string{"nfs"}, nil))
	wantErr := errors.New("collect failed")

	got, err := a.withIgnoreFilter(staticCollect(nil, wantErr))(context.Background())
	if !errors.Is(err, wantErr) {
		t.Errorf("err = %v, want %v", err, wantErr)
	}
	if got != nil {
		t.Errorf("got %v, want nil metrics", got)
	}
}
