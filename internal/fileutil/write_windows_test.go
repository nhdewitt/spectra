//go:build windows

package fileutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func isElevated() bool {
	cmd := exec.Command("net", "session")
	err := cmd.Run()
	return err == nil
}

func getFileSIDs(t *testing.T, path string) string {
	psCmd := fmt.Sprintf(`(Get-Acl %s).Access | ForEach-Object { $_.IdentityReference.Translate([System.Security.Principal.SecurityIdentifier]).Value }`, path)

	cmd := exec.Command("pwsh", "-NoProfile", "-NonInteractive", "-Command", psCmd)

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to read ACLs via pwsh: %v\nOutput: %s", err, string(out))
	}
}

func TestWriteSecure(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping WriteSecure: requires admin privileges")
	}
	if !isElevated() {
		t.Skip("Skipping WriteSecure test: must run as Administrator on Windows to manipulate ACLs.")
	}

	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "agent.json")
	secretData := []byte(`{"secret": "windows-test-123"}`)

	err := WriteSecure(filePath, secretData)
	if err != nil {
		t.Fatalf("WriteSecure failed: %v", err)
	}

	sids := getFileSIDs(t, filePath)

	if !strings.Contains(sids, "S-1-5-18") {
		t.Error("SYSTEM (S-1-5-18) should have access")
	}
	if !strings.Contains(sids, "S-1-5-32-544") {
		t.Error("Administrators (S-1-5-32-544) should have access")
	}
	if strings.Contains(sids, "S-1-1-0") {
		t.Error("Everyone (S-1-1-0) should not have access")
	}
}
