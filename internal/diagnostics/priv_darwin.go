//go:build darwin

package diagnostics

func isPrivileged() bool {
	return true
}
