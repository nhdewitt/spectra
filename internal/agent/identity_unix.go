//go:build linux || freebsd

package agent

func identityPath() string {
	return "/etc/spectra/agent-id.json"
}
