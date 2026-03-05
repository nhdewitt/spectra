//go:build linux || freebsd || darwin

package agent

func identityPath() string {
	return "/etc/spectra/agent-id.json"
}
