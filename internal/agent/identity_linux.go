//go:build linux

package agent

func identityPath() string {
	return "/etc/spectra/agent-id.json"
}
