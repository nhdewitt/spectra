package diagnostics

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
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

func TestRunDiskUsageTop_SingleFile(t *testing.T) {
	rootDir := t.TempDir()
	createDummyFile(t, filepath.Join(rootDir, "only.txt"), 42)

	ctx := context.Background()
	report, err := RunDiskUsageTop(ctx, rootDir, 10, 10)
	if err != nil {
		t.Fatalf("failed: %v", err)
	}

	if report.ScannedFiles != 1 {
		t.Errorf("expected 1 scanned file, got %d", report.ScannedFiles)
	}
	if len(report.TopFiles) != 1 {
		t.Fatalf("expected 1 top file, got %d", len(report.TopFiles))
	}
	if report.TopFiles[0].Size != 42 {
		t.Errorf("expected file size 42, got %d", report.TopFiles[0].Size)
	}
}

func TestRunDiskUsageTop_DeepNesting(t *testing.T) {
	rootDir := t.TempDir()

	deepPath := filepath.Join(rootDir, "a", "b", "c", "d", "e")
	createDummyFile(t, filepath.Join(deepPath, "deep.txt"), 999)

	ctx := context.Background()
	report, err := RunDiskUsageTop(ctx, rootDir, 10, 10)
	if err != nil {
		t.Fatalf("failed: %v", err)
	}

	if report.ScannedFiles != 1 {
		t.Errorf("expected 1 scanned file, got %d", report.ScannedFiles)
	}
	if report.ScannedDirs != 6 {
		t.Errorf("expected 6 scanned dirs (root/a/b/c/d/e), got %d", report.ScannedDirs)
	}

	for _, d := range report.TopDirs {
		if d.Size != 999 {
			t.Errorf("expected dir size 999, got %d for %s", d.Size, d.Path)
		}
	}
}

func TestRunDiskUsageTop_ZeroSizeFiles(t *testing.T) {
	rootDir := t.TempDir()

	createDummyFile(t, filepath.Join(rootDir, "empty1.txt"), 0)
	createDummyFile(t, filepath.Join(rootDir, "empty2.txt"), 0)
	createDummyFile(t, filepath.Join(rootDir, "notempty.txt"), 100)

	ctx := context.Background()
	report, err := RunDiskUsageTop(ctx, rootDir, 10, 10)
	if err != nil {
		t.Fatalf("failed: %v", err)
	}

	if report.ScannedFiles != 3 {
		t.Errorf("expected 3 scanned files, got %d", report.ScannedFiles)
	}
	if len(report.TopFiles) != 3 {
		t.Errorf("expected 3 top files, got %d", len(report.TopFiles))
	}
}

func TestRunDiskUsageTop_ManyFiles(t *testing.T) {
	rootDir := t.TempDir()

	for i := range 100 {
		name := fmt.Sprintf("file%03d.txt", i)
		createDummyFile(t, filepath.Join(rootDir, name), int64(i*10))
	}

	ctx := context.Background()
	report, err := RunDiskUsageTop(ctx, rootDir, 5, 5)
	if err != nil {
		t.Fatalf("failed: %v", err)
	}

	if report.ScannedFiles != 100 {
		t.Errorf("expected 100 scanned files, got %d", report.ScannedFiles)
	}
	if len(report.TopFiles) != 5 {
		t.Errorf("expected 5 top files, got %d", len(report.TopFiles))
	}

	for i := range len(report.TopFiles) - 1 {
		if report.TopFiles[i].Size < report.TopFiles[i+1].Size {
			t.Errorf("top files not in descending order: %d < %d", report.TopFiles[i].Size, report.TopFiles[i+1].Size)
		}
	}

	if report.TopFiles[0].Size != 990 {
		t.Errorf("expected largest file size 990, got %d", report.TopFiles[0].Size)
	}
}

func TestRunDiskUsageTop_ManyDirectories(t *testing.T) {
	rootDir := t.TempDir()

	for i := range 20 {
		dirName := fmt.Sprintf("dir%02d", i)
		dirPath := filepath.Join(rootDir, dirName)
		for j := range i {
			createDummyFile(t, filepath.Join(dirPath, fmt.Sprintf("f%d.txt", j)), 100)
		}
	}

	ctx := context.Background()
	report, err := RunDiskUsageTop(ctx, rootDir, 5, 100)
	if err != nil {
		t.Fatalf("failed: %v", err)
	}

	if len(report.TopDirs) != 5 {
		t.Errorf("expected 5 top dirs, got %d", len(report.TopDirs))
	}

	for i := range len(report.TopDirs) - 1 {
		if report.TopDirs[i].Size < report.TopDirs[i+1].Size {
			t.Errorf("top dirs not in descending order: %d < %d", report.TopDirs[i].Size, report.TopDirs[i+1].Size)
		}
	}
}

func TestRunDiskUsageTop_FileCount(t *testing.T) {
	rootDir := t.TempDir()

	subDir := filepath.Join(rootDir, "multi")
	for i := range 10 {
		createDummyFile(t, filepath.Join(subDir, fmt.Sprintf("f%d.txt", i)), 50)
	}

	ctx := context.Background()
	report, err := RunDiskUsageTop(ctx, rootDir, 10, 10)
	if err != nil {
		t.Fatalf("failed: %v", err)
	}

	var multiDir *struct {
		size, count uint64
	}
	for _, d := range report.TopDirs {
		if filepath.Base(d.Path) == "multi" {
			multiDir = &struct {
				size  uint64
				count uint64
			}{d.Size, d.Count}
			break
		}
	}

	if multiDir == nil {
		t.Fatal("multi directory not found in top dirs")
	}
	if multiDir.count != 10 {
		t.Errorf("expected 10 files in multi dir, got %d", multiDir.count)
	}
	if multiDir.size != 500 {
		t.Errorf("expected size 500 in multi dir, got %d", multiDir.size)
	}
}

func TestRunDiskUsageTop_ReportFields(t *testing.T) {
	rootDir := t.TempDir()
	createDummyFile(t, filepath.Join(rootDir, "test.txt"), 100)

	ctx := context.Background()
	before := time.Now()
	report, err := RunDiskUsageTop(ctx, rootDir, 10, 10)
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	after := time.Now()

	if report.Root != rootDir {
		t.Errorf("expected root %s, got %s", rootDir, report.Root)
	}
	if report.DurationMs < 0 {
		t.Errorf("duration should be non-negative, got %d", report.DurationMs)
	}
	if report.ScannedAt.Before(before) || report.ScannedAt.After(after) {
		t.Errorf("ScannedAt %v not between %v and %v", report.ScannedAt, before, after)
	}
}

func TestRunDiskUsageTop_NonExistentPath(t *testing.T) {
	ctx := context.Background()
	report, err := RunDiskUsageTop(ctx, "/nonexistent/path/that/does/not/exist", 10, 10)
	if err != nil {
		return
	}

	if report.ErrorCount == 0 {
		t.Error("expected error count > 0 for non-existent path")
	}
}

func TestRunDiskUsageTop_ContextTimeout(t *testing.T) {
	rootDir := t.TempDir()

	for i := range 50 {
		for j := range 50 {
			path := filepath.Join(rootDir, fmt.Sprintf("d%d", i), fmt.Sprintf("f%d.txt", j))
			createDummyFile(t, path, 100)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	time.Sleep(1 * time.Millisecond)

	_, err := RunDiskUsageTop(ctx, rootDir, 10, 10)
	if err != context.DeadlineExceeded {
		t.Logf("got error: %v", err)
	}
}

func TestRunDiskUsageTop_TopNZero(t *testing.T) {
	rootDir := t.TempDir()
	createDummyFile(t, filepath.Join(rootDir, "test.txt"), 100)

	ctx := context.Background()

	report, err := RunDiskUsageTop(ctx, rootDir, 0, 0)
	if err != nil {
		t.Fatalf("failed: %v", err)
	}

	if report.ScannedFiles != 1 {
		t.Errorf("expected 1 scanned file, got %d", report.ScannedFiles)
	}
	if len(report.TopFiles) != 0 {
		t.Errorf("expected 0 top files, got %d", len(report.TopFiles))
	}
	if len(report.TopDirs) != 0 {
		t.Errorf("expected 0 top dirs, got %d", len(report.TopDirs))
	}
}

func TestRunDiskUsageTop_SameSize(t *testing.T) {
	rootDir := t.TempDir()

	for i := range 5 {
		createDummyFile(t, filepath.Join(rootDir, fmt.Sprintf("same%d.txt", i)), 100)
	}

	ctx := context.Background()
	report, err := RunDiskUsageTop(ctx, rootDir, 10, 3)
	if err != nil {
		t.Fatalf("failed: %v", err)
	}

	if len(report.TopFiles) != 3 {
		t.Errorf("expected 3 top files, got %d", len(report.TopFiles))
	}

	for _, f := range report.TopFiles {
		if f.Size != 100 {
			t.Errorf("expected size 100, got %d", f.Size)
		}
	}
}

func BenchmarkRunDiskUsageTop_Small(b *testing.B) {
	rootDir := b.TempDir()

	for i := range 5 {
		createDummyFileB(b, filepath.Join(rootDir, fmt.Sprintf("f%d.txt", i)), 1000)
	}
	for i := range 5 {
		createDummyFileB(b, filepath.Join(rootDir, "sub", fmt.Sprintf("f%d.txt", i)), 1000)
	}

	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = RunDiskUsageTop(ctx, rootDir, 10, 10)
	}
}

func BenchmarkRunDiskUsageTop_Medium(b *testing.B) {
	rootDir := b.TempDir()

	for i := range 10 {
		for j := range 10 {
			path := filepath.Join(rootDir, fmt.Sprintf("dir%d", i), fmt.Sprintf("f%d.txt", j))
			createDummyFileB(b, path, 1000)
		}
	}

	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = RunDiskUsageTop(ctx, rootDir, 10, 10)
	}
}

func BenchmarkRunDiskUsageTop_Large(b *testing.B) {
	rootDir := b.TempDir()

	for i := range 100 {
		for j := range 10 {
			path := filepath.Join(rootDir, fmt.Sprintf("dir%02d", i), fmt.Sprintf("f%d.txt", j))
			createDummyFileB(b, path, 1000)
		}
	}

	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = RunDiskUsageTop(ctx, rootDir, 10, 10)
	}
}

func BenchmarkRunDiskUsageTop_Deep(b *testing.B) {
	rootDir := b.TempDir()

	path := rootDir
	for i := range 20 {
		path = filepath.Join(path, fmt.Sprintf("d%d", i))
		for j := range 5 {
			createDummyFileB(b, filepath.Join(path, fmt.Sprintf("f%d.txt", j)), 1000)
		}
	}

	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = RunDiskUsageTop(ctx, rootDir, 10, 10)
	}
}

func BenchmarkRunDiskUsageTop_Wide(b *testing.B) {
	rootDir := b.TempDir()

	for i := range 500 {
		createDummyFileB(b, filepath.Join(rootDir, fmt.Sprintf("f%03d.txt", i)), 1000)
	}

	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = RunDiskUsageTop(ctx, rootDir, 10, 10)
	}
}

func BenchmarkRunDiskUsageTop_VaryingTopN(b *testing.B) {
	rootDir := b.TempDir()

	for i := range 50 {
		for j := range 20 {
			path := filepath.Join(rootDir, fmt.Sprintf("dir%02d", i), fmt.Sprintf("f%d.txt", j))
			createDummyFileB(b, path, int64((i+1)*(j+1)*10))
		}
	}

	ctx := context.Background()

	for _, topN := range []int{5, 10, 25, 50, 100} {
		b.Run(fmt.Sprintf("top%d", topN), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				_, _ = RunDiskUsageTop(ctx, rootDir, topN, topN)
			}
		})
	}
}

func BenchmarkRunDiskUsageTop_RealFS(b *testing.B) {
	// Benchmark against a real filesystem path
	// Use /tmp which should exist and have varied content
	root := os.TempDir()

	ctx := context.Background()

	// First, see how many files we're dealing with
	report, err := RunDiskUsageTop(ctx, root, 10, 10)
	if err != nil {
		b.Skipf("cannot scan %s: %v", root, err)
	}
	b.Logf("Benchmarking %s: %d files, %d dirs", root, report.ScannedFiles, report.ScannedDirs)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = RunDiskUsageTop(ctx, root, 10, 10)
	}
}

func createDummyFileB(b *testing.B, path string, size int64) {
	b.Helper()

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		b.Fatalf("failed to create dir %s: %v", dir, err)
	}

	f, err := os.Create(path)
	if err != nil {
		b.Fatalf("failed to create file %s: %v", path, err)
	}
	defer f.Close()

	if err := f.Truncate(size); err != nil {
		b.Fatalf("failed to truncate file: %s: %v", path, err)
	}
}
