package fileutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteSecure_Contents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.json")

	if err := WriteSecure(path, []byte("hello")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "hello" {
		t.Errorf("contents = %q, want hello", data)
	}
}
