//go:build linux || freebsd || darwin

package memory

// totalPhysical reads total physical memory from the platform. Every
// non-Windows target the agent builds for supplies parseMemInfo:
//   - linux from /proc
//   - freebsd from sysctl
//   - darwin from sysctl
func totalPhysical() (uint64, error) {
	raw, err := parseMemInfo()
	if err != nil {
		return 0, err
	}
	return raw.Total, nil
}
