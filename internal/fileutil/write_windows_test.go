//go:build windows

package fileutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func isElevated() bool {
	f, err := os.Open(`\\.\PHYSICALDRIVE0`)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

func TestWriteSecure(t *testing.T) {
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

	readData, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if string(readData) != string(secretData) {
		t.Errorf("Expected %s, got %s", secretData, readData)
	}

	out, err := exec.Command("icacls", filePath).Output()
	if err != nil {
		t.Fatalf("icacls failed: %v", err)
	}

	output := string(out)
	if !strings.Contains(output, "S-1-5-18") {
		t.Error("SYSTEM (S-1-5-18) should have access")
	}
	if !strings.Contains(output, "S-1-5-32-544") {
		t.Error("Administrators (S-1-5-32-544) should have access")
	}
	if strings.Contains(output, "S-1-1-0") {
		t.Error("Everyone (S-1-1-0) should not have access")
	}
}
