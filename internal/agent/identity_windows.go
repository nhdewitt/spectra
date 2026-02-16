//go:build windows

package agent

import (
	"os"
	"path/filepath"
)

func identityPath() string {
	return filepath.Join(os.Getenv("ProgramData"), "Spectra", "agent-id.json")
}
