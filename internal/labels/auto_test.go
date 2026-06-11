package labels

import "testing"

func TestComputeAutoLabels(t *testing.T) {
	tests := []struct {
		name string
		info AgentInfo
		want []Label
	}{
		{
			name: "all_fields",
			info: AgentInfo{
				OS:           "linux",
				Arch:         "arm64",
				Hardware:     "raspberry-pi",
				AgentVersion: "1.4.2",
			},
			want: []Label{
				{"os", "linux"},
				{"arch", "arm64"},
				{"hardware", "raspberry-pi"},
				{"agent_version", "1.4.2"},
			},
		},
		{
			name: "no_platform",
			info: AgentInfo{
				OS:           "linux",
				Arch:         "amd64",
				AgentVersion: "1.4.2",
			},
			want: []Label{
				{"os", "linux"},
				{"arch", "amd64"},
				{"agent_version", "1.4.2"},
			},
		},
		{
			name: "freebsd",
			info: AgentInfo{
				OS:           "freebsd",
				Arch:         "amd64",
				AgentVersion: "1.4.2",
			},
			want: []Label{
				{"os", "freebsd"},
				{"arch", "amd64"},
				{"agent_version", "1.4.2"},
			},
		},
		{
			name: "empty_input",
			info: AgentInfo{},
			want: []Label{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeAutoLabels(tt.info)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d (got %v)", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("[%d] = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// TestAutoLabelKeysAreReserved enforces the invariant that every key
// emitted by ComputeAutoLabels appears in ReservedKeys. If this test fails,
// either remove the key from ComputeAutoLabels or add it to ReservedKeys -
// the two must stay in lockstep, otherwise user writes can clobber auto
// labels.
func TestAutoLabelKeysAreReserved(t *testing.T) {
	full := AgentInfo{
		OS:           "x",
		Arch:         "x",
		Hardware:     "x",
		AgentVersion: "x",
	}
	for _, l := range ComputeAutoLabels(full) {
		if !IsReservedKey(l.Key) {
			t.Errorf("ComputeAutoLabels emits key %q which is not in ReservedKeys", l.Key)
		}
	}
}

func TestUnpack(t *testing.T) {
	labels := []Label{
		{"os", "linux"},
		{"arch", "amd64"},
	}
	keys, values := Unpack(labels)
	if len(keys) != 2 || len(values) != 2 {
		t.Fatalf("len = (%d, %d), want (2, 2)", len(keys), len(values))
	}
	if keys[0] != "os" || keys[1] != "arch" {
		t.Errorf("keys = %v, want [os arch]", keys)
	}
	if values[0] != "linux" || values[1] != "amd64" {
		t.Errorf("values = %v, want [linux amd64]", values)
	}
}

func TestUnpack_Empty(t *testing.T) {
	// Empty input must produce non-nil zero-length slices (so pgx marshals
	// them to empty PG arrays rather than NULL).
	keys, values := Unpack(nil)
	if keys == nil || values == nil {
		t.Errorf("expected non-nil slices, got keys=%v values=%v", keys, values)
	}
	if len(keys) != 0 || len(values) != 0 {
		t.Errorf("expected zero-length, got len(keys)=%d len(values)=%d", len(keys), len(values))
	}
}
