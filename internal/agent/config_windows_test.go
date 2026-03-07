//go:build windows

package agent

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveCredentials_Permissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.json")

	SaveCredentials(path, "https://example.com", "id", "secret")

	out, err := exec.Command("icacls", path).Output()
	if err != nil {
		t.Fatalf("icacls failed: %v", err)
	}

	output := string(out)
	if !strings.Contains(output, "SYSTEM") {
		t.Error("SYSTEM should have access")
	}
	if !strings.Contains(output, "Administrators") {
		t.Error("Administrators should have access")
	}
	if strings.Contains(output, "Everyone") {
		t.Error("Everyone should not have access")
	}
}
