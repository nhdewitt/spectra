//go:build !windows

package fileutil

import (
	"os"
	"path/filepath"
)

// WriteSecure writes data atomically with owner-only read/write permissions.
func WriteSecure(path string, data []byte) error {
	dir := filepath.Dir(path)

	tmpFile, err := os.CreateTemp(dir, "agent-config-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmpFile.Name()

	defer os.Remove(tmpName)

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return err
	}

	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		return err
	}

	if err := tmpFile.Close(); err != nil {
		return err
	}

	if err := os.Chmod(tmpName, 0600); err != nil {
		return err
	}

	return os.Rename(tmpName, path)
}
