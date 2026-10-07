package agent

import (
	"context"
	"maps"

	"github.com/nhdewitt/spectra/internal/collector"
	"github.com/nhdewitt/spectra/internal/protocol"
)

// ignoreLists holds the filesystem types and network interfaces an admin has
// told this agent to stop reporting, from the ignored_filesystems and
// ignored_interfaces remote config keys. Each config poll replaces it whole.
type ignoreLists struct {
	filesystems map[string]struct{}
	interfaces  map[string]struct{}
}

func newIgnoreLists(filesystems, interfaces []string) *ignoreLists {
	ig := &ignoreLists{
		filesystems: make(map[string]struct{}, len(filesystems)),
		interfaces:  make(map[string]struct{}, len(interfaces)),
	}
	for _, f := range filesystems {
		ig.filesystems[f] = struct{}{}
	}
	for _, i := range interfaces {
		ig.interfaces[i] = struct{}{}
	}
	return ig
}

func (ig *ignoreLists) equal(other *ignoreLists) bool {
	return maps.Equal(ig.filesystems, other.filesystems) && maps.Equal(ig.interfaces, other.interfaces)
}

func (ig *ignoreLists) drops(m protocol.Metric) bool {
	switch v := m.(type) {
	case protocol.DiskMetric:
		_, ok := ig.filesystems[v.Filesystem]
		return ok
	case protocol.NetworkMetric:
		_, ok := ig.interfaces[v.Interface]
		return ok
	}
	return false
}

// withIgnoreFilter drops metrics for ignored filesystems and interfaces from fn's results,
// reading the lists current at each collection.
func (a *Agent) withIgnoreFilter(fn collector.CollectFunc) collector.CollectFunc {
	return func(ctx context.Context) ([]protocol.Metric, error) {
		metrics, err := fn(ctx)
		ig := a.ignore.Load()
		if err != nil || ig == nil {
			return metrics, err
		}

		kept := metrics[:0]
		for _, m := range metrics {
			if !ig.drops(m) {
				kept = append(kept, m)
			}
		}
		return kept, nil
	}
}
