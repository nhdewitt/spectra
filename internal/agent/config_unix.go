//go:build !windows

package agent

import "os"

func writeSecureFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0600)
}
