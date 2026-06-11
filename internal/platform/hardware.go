package platform

// HardwareClass returns the agent's hardware category. Safe to call once
// at startup and cache; the value doesn't change at runtime.
func HardwareClass() string {
	if isRaspberryPi() {
		return "raspberry-pi"
	}
	if isContainer() {
		return "container"
	}
	if isVirtualMachine() {
		return "virtual-machine"
	}
	return "bare-metal"
}
