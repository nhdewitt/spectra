//go:build linux || freebsd

package diagnostics

import "os"

func isPrivileged() bool {
	return os.Geteuid() == 0
}
