//go:build windows

package agent

import (
	"os"
	"os/exec"
)

func writeSecureFile(path string, data []byte) error {
	if err := os.WriteFile(path, data, 0600); err != nil {
		return err
	}

	// Strip inherited permissions, grant only system+admins
	cmds := [][]string{
		{"icacls", path, "/inheritance:r"},
		{"icacls", path, "/grant", "SYSTEM:(F)"},
		{"icacls", path, "/grant", "Administrators:(F)"},
	}

	for _, cmd := range cmds {
		if err := exec.Command(cmd[0], cmd[1:]...).Run(); err != nil {
			return err
		}
	}

	return nil
}
