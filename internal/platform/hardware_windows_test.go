//go:build windows

package platform

import "testing"

func TestMatchVMIndicator(t *testing.T) {
	tests := []struct {
		name string
		val  string
		want bool
	}{
		// The fleet runs Proxmox, so these two are the ones that matter.
		{name: "ProxmoxSystemProductName", val: "Standard PC (Q35 + ICH9, 2009)", want: false},
		{name: "ProxmoxManufacturer", val: "QEMU", want: true},
		{name: "KVMProductName", val: "KVM", want: true},

		{name: "VMwareProduct", val: "VMware Virtual Platform", want: true},
		{name: "VMware7", val: "VMware7,1", want: true},
		{name: "VirtualBoxManufacturer", val: "innotek GmbH", want: true},
		{name: "VirtualBoxProduct", val: "VirtualBox", want: true},
		{name: "HyperV", val: "Virtual Machine", want: true},
		{name: "Parallels", val: "Parallels Virtual Platform", want: true},
		{name: "EC2", val: "Amazon EC2", want: true},
		{name: "GCE", val: "Google Compute Engine", want: true},
		{name: "Bhyve", val: "BHYVE", want: true},

		{name: "BareMetalDell", val: "OptiPlex 7090", want: false},
		{name: "BareMetalDellManufacturer", val: "Dell Inc.", want: false},
		{name: "BareMetalLenovo", val: "LENOVO", want: false},
		{name: "BareMetalHP", val: "HP EliteDesk 800 G6", want: false},
		{name: "Empty", val: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchVMIndicator(tt.val, vmIndicators); got != tt.want {
				t.Errorf("matchVMIndicator(%q) = %v, want %v", tt.val, got, tt.want)
			}
		})
	}
}

// A Proxmox Windows guest reports "Standard PC (Q35 + ICH9, 2009)" as
// SystemProductName, which matches nothing, and "QEMU" as SystemManufacturer,
// which does. Detection depends on the second key being read, so a change
// that stops at the first miss would silently misclassify the fleet.
func TestMatchVMIndicator_ProxmoxNeedsManufacturer(t *testing.T) {
	if matchVMIndicator("Standard PC (Q35 + ICH9, 2009)", vmIndicators) {
		t.Error("the Q35 product name is not expected to match on its own")
	}
	if !matchVMIndicator("QEMU", vmIndicators) {
		t.Fatal("QEMU manufacturer must match, or Proxmox guests read as bare metal")
	}
}

// "virtual machine" is broad enough to match a value that merely contains the
// phrase. Hyper-V genuinely reports it, so the entry has to stay, but a
// physical host with those words in its product name would be misclassified.
func TestMatchVMIndicator_VirtualMachineIsBroad(t *testing.T) {
	if !matchVMIndicator("Virtual Machine", vmIndicators) {
		t.Error("Hyper-V guests report exactly this and must match")
	}
	if !matchVMIndicator("Virtual Machine Host Workstation", vmIndicators) {
		t.Skip("substring matching classifies this as a VM; documented, not asserted")
	}
}

func TestMatchVMIndicator_CaseInsensitive(t *testing.T) {
	for _, val := range []string{"qemu", "QEMU", "QeMu"} {
		if !matchVMIndicator(val, vmIndicators) {
			t.Errorf("matchVMIndicator(%q) = false, want true", val)
		}
	}
}

func TestIsRaspberryPiAndIsContainer_AreFalseOnWindows(t *testing.T) {
	if isRaspberryPi() {
		t.Error("isRaspberryPi must be false on Windows")
	}
	if isContainer() {
		t.Error("isContainer must be false on Windows: Windows containers are not detected")
	}
}

func TestFindPowerShell(t *testing.T) {
	path := findPowerShell()

	// Windows Server Core images ship neither, so an empty result is valid.
	if path == "" {
		t.Log("neither pwsh nor powershell is on PATH")
		return
	}
	t.Logf("PowerShell resolved to %q", path)
}
