package server

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// chtimeSeq gives each write a distinct mtime. Deriving the timestamp from the
// file's own current mtime fails on filesystems with coarse timestamp
// granularity (WSL over DrvFs, some network mounts): two writes microseconds
// apart read back the same value, so the forced mtimes come out identical and
// refresh correctly decides nothing changed.
var chtimeSeq int64

func writeChecksums(t *testing.T, dir, contents string) {
	t.Helper()

	path := filepath.Join(dir, "checksums.sha256")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write checksums: %v", err)
	}

	chtimeSeq++
	stamp := time.Now().Add(time.Duration(chtimeSeq) * time.Minute)
	if err := os.Chtimes(path, stamp, stamp); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
}

// TestReleaseManifest_PicksUpNewChecksumsWithoutRestart is the regression test
// for a real incident: the manifest loaded once at construction, so a
// deploy-releases push (deliberately separate from deploy-server, precisely so
// it needs no restart) silently had no effect until the next restart. The
// overview flagged agents as outdated against the stale hash while an update
// push sent that same stale hash, which the agent already matched.
func TestReleaseManifest_PicksUpNewChecksumsWithoutRestart(t *testing.T) {
	dir := t.TempDir()
	writeChecksums(t, dir, "aaa111  spectra-agent-linux-amd64\n")

	rm := newReleaseManifest(dir)

	if got := rm.expectedHash("linux", "amd64"); got != "aaa111" {
		t.Fatalf("initial hash: got %q, want aaa111", got)
	}

	// Simulate `make deploy-releases` against the running server.
	writeChecksums(t, dir, "bbb222  spectra-agent-linux-amd64\n")

	if got := rm.expectedHash("linux", "amd64"); got != "bbb222" {
		t.Errorf("hash after redeploy: got %q, want bbb222 without a restart", got)
	}
	if got, ok := rm.get("spectra-agent-linux-amd64"); !ok || got != "bbb222" {
		t.Errorf("get after redeploy: got %q (ok=%v), want bbb222", got, ok)
	}
}

// TestReleaseManifest_DropsRemovedEntries covers the other half of a reload:
// merging into the existing map would keep serving a filename that is no
// longer in the manifest.
func TestReleaseManifest_DropsRemovedEntries(t *testing.T) {
	dir := t.TempDir()
	writeChecksums(t, dir,
		"aaa111  spectra-agent-linux-amd64\nccc333  spectra-agent-linux-arm64\n")

	rm := newReleaseManifest(dir)
	if _, ok := rm.get("spectra-agent-linux-arm64"); !ok {
		t.Fatal("arm64 entry missing from the initial load")
	}

	writeChecksums(t, dir, "aaa111  spectra-agent-linux-amd64\n")

	if _, ok := rm.get("spectra-agent-linux-arm64"); ok {
		t.Error("arm64 entry survived a manifest that no longer lists it")
	}
	if got, ok := rm.get("spectra-agent-linux-amd64"); !ok || got != "aaa111" {
		t.Errorf("amd64 entry: got %q (ok=%v), want aaa111", got, ok)
	}
}

// TestReleaseManifest_UnchangedFileIsNotReread pins the mtime guard.
// expectedHash runs once per agent in the fleet-overview loop, so an
// unconditional re-read would parse this file once per agent per request.
func TestReleaseManifest_UnchangedFileIsNotReread(t *testing.T) {
	dir := t.TempDir()
	writeChecksums(t, dir, "aaa111  spectra-agent-linux-amd64\n")

	rm := newReleaseManifest(dir)
	rm.expectedHash("linux", "amd64")

	// Replace the contents while keeping the recorded mtime, which is the only
	// thing refresh consults. A re-read would pick up the new value.
	path := filepath.Join(dir, "checksums.sha256")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if err := os.WriteFile(path, []byte("zzz999  spectra-agent-linux-amd64\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	if got := rm.expectedHash("linux", "amd64"); got != "aaa111" {
		t.Errorf("hash: got %q, want aaa111: an unchanged mtime should skip the re-read", got)
	}
}

// TestReleaseManifest_SurvivesMissingChecksumsFile covers the transient case of
// reading while deploy-releases is mid-copy. Callers keep the last good load
// rather than losing every entry.
func TestReleaseManifest_SurvivesMissingChecksumsFile(t *testing.T) {
	dir := t.TempDir()
	writeChecksums(t, dir, "aaa111  spectra-agent-linux-amd64\n")

	rm := newReleaseManifest(dir)
	rm.expectedHash("linux", "amd64")

	if err := os.Remove(filepath.Join(dir, "checksums.sha256")); err != nil {
		t.Fatalf("remove: %v", err)
	}

	if got := rm.expectedHash("linux", "amd64"); got != "aaa111" {
		t.Errorf("hash: got %q, want the last good value aaa111", got)
	}
}

func TestReleaseManifest_NoReleaseDir(t *testing.T) {
	rm := newReleaseManifest("")

	// Must not panic, and must report nothing as available.
	if got := rm.expectedHash("linux", "amd64"); got != "" {
		t.Errorf("hash: got %q, want empty", got)
	}
	if platforms := rm.availablePlatforms(); len(platforms) != 0 {
		t.Errorf("platforms: got %d, want 0", len(platforms))
	}
}

func TestReleaseManifest_AvailablePlatformsNeedsBinaryOnDisk(t *testing.T) {
	dir := t.TempDir()
	writeChecksums(t, dir,
		"aaa111  spectra-agent-linux-amd64\nccc333  spectra-agent-linux-arm64\n")

	// Only one of the two binaries actually exists.
	if err := os.WriteFile(filepath.Join(dir, "spectra-agent-linux-amd64"), []byte("binary"), 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}

	rm := newReleaseManifest(dir)
	platforms := rm.availablePlatforms()

	if len(platforms) != 1 {
		t.Fatalf("platforms: got %d, want 1", len(platforms))
	}
	if platforms[0].Filename != "spectra-agent-linux-amd64" {
		t.Errorf("platform: got %q, want spectra-agent-linux-amd64", platforms[0].Filename)
	}
}
