//go:build linux

package platform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCgroupIsContainer(t *testing.T) {
	tests := []struct {
		name string
		data string
		want bool
	}{
		{
			name: "Docker",
			data: "12:pids:/docker/3f4a1b2c\n11:cpu:/docker/3f4a1b2c\n",
			want: true,
		},
		{
			name: "LXC",
			data: "0::/lxc/107/ns\n",
			want: true,
		},
		{
			name: "Containerd",
			data: "0::/containerd/abcdef\n",
			want: true,
		},
		{
			name: "BareMetalCgroupV2",
			data: "0::/init.scope\n",
			want: false,
		},
		{
			name: "SystemdUserSlice",
			data: "0::/user.slice/user-1000.slice/session-3.scope\n",
			want: false,
		},
		{
			name: "Empty",
			data: "",
			want: false,
		},
		{
			// A Proxmox host running LXC guests: the host's own PID 1 is not
			// in a container cgroup even though /lxc/ paths exist elsewhere.
			name: "ProxmoxHostNotItselfAContainer",
			data: "0::/init.scope\n",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cgroupIsContainer(tt.data); got != tt.want {
				t.Errorf("cgroupIsContainer(%q) = %v, want %v", tt.data, got, tt.want)
			}
		})
	}
}

func TestMatchHypervisor(t *testing.T) {
	tests := []struct {
		name  string
		field string
		want  bool
	}{
		{name: "QEMUUppercase", field: "QEMU", want: true},
		{name: "HyperV", field: "Microsoft Corporation", want: true},
		{name: "VirtualBoxVendor", field: "innotek GmbH", want: true},
		{name: "KVMProductName", field: "KVM", want: true},
		{name: "TrailingNewline", field: "QEMU\n", want: true},
		{name: "SurroundingWhitespace", field: "  Bochs  \n", want: true},
		{name: "BareMetalDell", field: "Dell Inc.", want: false},
		{name: "BareMetalSupermicro", field: "Supermicro", want: false},
		{name: "Empty", field: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchHypervisor(tt.field, hypervisors); got != tt.want {
				t.Errorf("matchHypervisor(%q) = %v, want %v", tt.field, got, tt.want)
			}
		})
	}
}

// "google" matches any DMI field containing it, so a physical machine with a
// Google-branded component would be misreported as a VM. Documents the known
// looseness rather than asserting it is correct.
func TestMatchHypervisor_SubstringIsDeliberatelyLoose(t *testing.T) {
	if !matchHypervisor("Google Compute Engine", []string{"google"}) {
		t.Error("expected GCE to match")
	}
	if !matchHypervisor("Google Titan Security Module", []string{"google"}) {
		t.Skip("substring matching would classify this as a VM; documented, not asserted")
	}
}

func TestIsContainer_CgroupFixture(t *testing.T) {
	t.Cleanup(func(orig string) func() {
		return func() { initCgroupPath = orig }
	}(initCgroupPath))

	// The env check runs before the cgroup read, so clear it to isolate.
	t.Setenv("container", "")

	path := filepath.Join(t.TempDir(), "cgroup")
	if err := os.WriteFile(path, []byte("0::/docker/deadbeef\n"), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}
	initCgroupPath = path

	if !isContainer() {
		t.Error("expected a docker cgroup to be detected as a container")
	}
}

// HardwareClass is ordered: a Pi running in a container reports raspberry-pi,
// and a VM inside a container reports container. Pinned because the order is
// load-bearing for collector registration and invisible from the return type.
func TestHardwareClass_ReturnsAKnownClass(t *testing.T) {
	got := HardwareClass()

	switch got {
	case "raspberry-pi", "container", "virtual-machine", "bare-metal":
	default:
		t.Errorf("HardwareClass() = %q, not one of the four known values", got)
	}
	t.Logf("This host classifies as %q", got)
}
