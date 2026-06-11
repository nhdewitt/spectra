package labels

// AgentInfo carries the agent metadata needed to derive auto labels.
// Callers map from their wire/protocol types to this sturct so the labels
// package stays free of protocol imports.
//
// Empty fields produce no corresponding label (ComputeAutoLabels strips them);
// ReplaceAutoLabels in turn diffs the resulting set against the DB and
// removes any auto labels no longer present.
type AgentInfo struct {
	OS           string // GOOS-style: "linux", "freebsd", "windows", "darwin"
	Arch         string // GOARCH-style: "amd64", "arm64", "armv61"
	Hardware     string // Board/chassis category: "raspberry-pi", "virtual-machine", "container", "bare-metal", ""
	AgentVersion string // from ldflags -X
}

// Label is a single key/value pair. The package returns []Label rather than
// map[string]string so insertion order is preserved.
type Label struct {
	Key, Value string
}

// ComputeAutoLabels returns the auto-label set for the given agent info.
//
// Invariant: every key returned here must be a member of ReservedKeys.
// Guarded by TestAutoLabelKeysAreReserved.
func ComputeAutoLabels(info AgentInfo) []Label {
	out := make([]Label, 0, 4)
	if info.OS != "" {
		out = append(out, Label{Key: "os", Value: info.OS})
	}
	if info.Arch != "" {
		out = append(out, Label{Key: "arch", Value: info.Arch})
	}
	if info.Hardware != "" {
		out = append(out, Label{Key: "hardware", Value: info.Hardware})
	}
	if info.AgentVersion != "" {
		out = append(out, Label{Key: "agent_version", Value: info.AgentVersion})
	}
	return out
}

// Unpack splits a []Label into parallel keys and values slices, the shape
// expected by the ReplaceAutoLabels sqlc query (text[]/text[] params).
// Returns non-nil zero-length slices for an empty input, which marshals
// to an empty PG array rather than NULL.
func Unpack(labels []Label) (keys, values []string) {
	keys = make([]string, len(labels))
	values = make([]string, len(labels))
	for i, l := range labels {
		keys[i] = l.Key
		values[i] = l.Value
	}
	return
}
