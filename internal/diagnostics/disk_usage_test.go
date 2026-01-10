package diagnostics

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func createDummyFile(t *testing.T, path string, size int64) {
	t.Helper()

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create dir %s: %v", dir, err)
	}

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create file %s: %v", path, err)
	}
	defer f.Close()

	if err := f.Truncate(size); err != nil {
		t.Fatalf("failed to truncate file: %s: %v", path, err)
	}
}

func TestRunDiskUsageTop_Basics(t *testing.T) {
	// Temp directory sandbox
	rootDir := t.TempDir()

	createDummyFile(t, filepath.Join(rootDir, "small.txt"), 100)
	createDummyFile(t, filepath.Join(rootDir, "sub", "medium.txt"), 500)
	createDummyFile(t, filepath.Join(rootDir, "sub", "large.txt"), 1000)

	ctx := context.Background()

	report, err := RunDiskUsageTop(ctx, rootDir, 2, 2)
	if err != nil {
		t.Fatalf("RunDiskUsageTop failed: %v", err)
	}

	if report.ScannedFiles != 3 {
		t.Errorf("expected 3 scanned files, got %d", report.ScannedFiles)
	}
	if report.ScannedDirs != 2 {
		t.Errorf("expected 2 scanned dirs, got %d", report.ScannedDirs)
	}

	if len(report.TopFiles) != 2 {
		t.Fatalf("expected 2 top files, got %d", len(report.TopFiles))
	}
	if report.TopFiles[0].Size != 1000 {
		t.Errorf("top file 1 should be 1000 bytes, got %d", report.TopFiles[0].Size)
	}
	if report.TopFiles[1].Size != 500 {
		t.Errorf("top file 2 should be 500 bytes, got %d", report.TopFiles[1].Size)
	}

	if len(report.TopDirs) != 2 {
		t.Fatalf("expected 2 top dirs, got %d", len(report.TopDirs))
	}

	if report.TopDirs[0].Size != 1600 {
		t.Errorf("expected root dir size 1600, got %d", report.TopDirs[0].Size)
	}
	if report.TopDirs[1].Size != 1500 {
		t.Errorf("expected sub dir size 1500, got %d", report.TopDirs[1].Size)
	}
}

func TestRunDiskUsageTop_Limits(t *testing.T) {
	rootDir := t.TempDir()

	for i := 1; i <= 5; i++ {
		name := fmt.Sprintf("file%d.txt", i)
		createDummyFile(t, filepath.Join(rootDir, name), int64(i*100))
	}

	ctx := context.Background()

	report, err := RunDiskUsageTop(ctx, rootDir, 5, 2)
	if err != nil {
		t.Fatalf("failed: %v", err)
	}

	if len(report.TopFiles) != 2 {
		t.Errorf("expected 2 top files, got %d", len(report.TopFiles))
	}

	if report.TopFiles[0].Size != 500 {
		t.Errorf("largest file should be 500, got %d", report.TopFiles[0].Size)
	}
}

func TestRunDiskUsageTop_Symlinks(t *testing.T) {
	rootDir := t.TempDir()
	realFile := filepath.Join(rootDir, "real.txt")
	createDummyFile(t, realFile, 1000)

	linkFile := filepath.Join(rootDir, "link.txt")
	if err := os.Symlink(realFile, linkFile); err != nil {
		t.Skipf("skipping symlink test: %v", err)
	}

	ctx := context.Background()

	report, err := RunDiskUsageTop(ctx, rootDir, 10, 10)
	if err != nil {
		t.Fatalf("failed: %v", err)
	}

	if report.ScannedFiles != 1 {
		t.Errorf("expected 1 scanned file (ignored symlinks), got %d", report.ScannedFiles)
	}

	if len(report.TopDirs) > 0 {
		if report.TopDirs[0].Size != 1000 {
			t.Errorf("expected total size 1000 (ignored symlinks), got %d", report.TopDirs[0].Size)
		}
	}
}

func TestRunDiskUsageTop_ContextCancel(t *testing.T) {
	rootDir := t.TempDir()
	createDummyFile(t, filepath.Join(rootDir, "f1"), 100)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := RunDiskUsageTop(ctx, rootDir, 10, 10)

	if err == nil {
		t.Error("expected error due to cancelled context, got nil")
	}
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}
