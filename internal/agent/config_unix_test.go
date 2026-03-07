//go:build !windows

package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveCredentials_Permissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.json")

	SaveCredentials(path, "https://example.com", "id", "secret")

	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Errorf("permissions = %o, want 0600", info.Mode().Perm())
	}
}
