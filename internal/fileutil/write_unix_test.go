//go:build !windows

package fileutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteSecure(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "agent.json")
	secretData := []byte(`{"secret": "test-123"}`)

	// 1. Write the file
	err := WriteSecure(filePath, secretData)
	if err != nil {
		t.Fatalf("WriteSecure failed: %v", err)
	}

	// 2. Verify the contents
	readData, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if string(readData) != string(secretData) {
		t.Errorf("Expected %s, got %s", secretData, readData)
	}

	// 3. Verify the 0600 permissions
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}

	// Perm() masks out the directory/symlink bits and just leaves the permission bits
	if info.Mode().Perm() != 0600 {
		t.Errorf("Expected file permissions to be 0600, got %04o", info.Mode().Perm())
	}
}
