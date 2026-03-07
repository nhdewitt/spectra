//go:build windows

package fileutil

import (
	"os"
	"os/exec"
	"path/filepath"
)

// WriteSecure writes data atomically and locks down permissions using Windows ACLs.
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
	tmpFile.Sync()
	tmpFile.Close()

	cmds := [][]string{
		// /inheritance:r removes all inherited permissions
		{"icacls", tmpName, "/inheritance:r"},

		// *S-1-5-18 - universal SID for "Local System"
		{"icacls", tmpName, "/grant", "*S-1-5-18:(F)"},

		// *S-1-5-32-544 - universal SID for "Administrators"
		{"icacls", tmpName, "/grant", "*S-1-5-32-544:(F)"},
	}

	for _, args := range cmds {
		if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
			return err
		}
	}

	return os.Rename(tmpName, path)
}
