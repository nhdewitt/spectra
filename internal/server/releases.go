package server

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// releaseManifest holds SHA256 checksums for pre-built agent binaries,
// re-read from checksums.sha256 whenever the file on disk changes.
//
// It must not cache indefinitely, since `make deploy-releases` is
// deliberately separate from `make deploy-server`, new agent binaries
// can be pushed without restarting the server. Loading once at
// construction meant the in-memory map silently drifted from disk until
// the next restart, and both readers of that meap then agreed with each
// other while disagreeing with reality.
type releaseManifest struct {
	mu         sync.RWMutex
	checksums  map[string]string // filename -> sha256
	releaseDir string

	// loadedModTime is the mtime of the checksums file the current map was
	// built from. expectedHash is called once per agent in the fleet-overview
	// loop, so refresh stats the file and only re-reads when it has
	// actually changed.
	loadedModTime time.Time
}

// platformInfo describes a downloadable agent build.
type platformInfo struct {
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Variant  string `json:"variant,omitempty"` // "armv6", "armv7"
	Label    string `json:"label"`             // user-friendly: "Raspberry Pi Zero/1 (armv6)"
	Filename string `json:"filename"`
}

// knownPlatforms defines all supported build targets and their display labels.
var knownPlatforms = []platformInfo{
	{
		OS:       "linux",
		Arch:     "amd64",
		Label:    "Linux (x86_64)",
		Filename: "spectra-agent-linux-amd64",
	},
	{
		OS:       "linux",
		Arch:     "arm64",
		Label:    "Linux (arm64)",
		Filename: "spectra-agent-linux-arm64",
	},
	{
		OS:       "linux",
		Arch:     "arm",
		Variant:  "armv6",
		Label:    "Raspberry Pi Zero/1 (armv6)",
		Filename: "spectra-agent-linux-armv6",
	},
	{
		OS:       "linux",
		Arch:     "arm",
		Variant:  "armv7",
		Label:    "Raspberry Pi 2/3/4 (armv7)",
		Filename: "spectra-agent-linux-armv7",
	},
	{
		OS:       "freebsd",
		Arch:     "amd64",
		Label:    "FreeBSD (x86_64)",
		Filename: "spectra-agent-freebsd-amd64",
	},
	{
		OS:       "darwin",
		Arch:     "amd64",
		Label:    "macOS (Intel)",
		Filename: "spectra-agent-darwin-amd64",
	},
	{
		OS:       "darwin",
		Arch:     "arm64",
		Label:    "macOS (Apple Silicon)",
		Filename: "spectra-agent-darwin-arm64",
	},
	{
		OS:       "windows",
		Arch:     "amd64",
		Label:    "Windows (x86_64)",
		Filename: "spectra-agent-windows-amd64.exe",
	},
}

// newReleaseManifest creates a manifest and loads checksums from the release directory.
// Returns nil if the directory doesn't exist or has no checksum file.
func newReleaseManifest(releaseDir string) *releaseManifest {
	rm := &releaseManifest{
		checksums:  make(map[string]string),
		releaseDir: releaseDir,
	}

	if releaseDir == "" {
		return rm
	}

	if err := rm.loadChecksums(rm.checksumPath()); err != nil {
		return rm
	}

	return rm
}

// checksumPath returns the path to the manifest file, or "" if this manifest
// has no release directory configured.
func (rm *releaseManifest) checksumPath() string {
	if rm.releaseDir == "" {
		return ""
	}
	return filepath.Join(rm.releaseDir, "checksums.sha256")
}

// refresh re-reads checksums.sha256 if it has changed since the last load.
//
// Every public accessor calls this first, so a deploy-release push takes
// effect on the next request rather than the next restart. A failed read is
// ignored: callers keep working against the last good load (which also
// covers the transient case of reading while deploy-releases is mid-copy),
// and the following request tries again.
func (rm *releaseManifest) refresh() {
	path := rm.checksumPath()
	if path == "" {
		return
	}

	info, err := os.Stat(path)
	if err != nil {
		return
	}

	rm.mu.RLock()
	unchanged := info.ModTime().Equal(rm.loadedModTime)
	rm.mu.RUnlock()
	if unchanged {
		return
	}

	_ = rm.loadChecksums(path)
}

// loadChecksums parses a checksums.sha256 file in the format:
//
// <hex>  <filename>
//
// The parsed entries replace the previous map rather than merging into
// it, so a filename dropped from the file stops being served rather
// than lingering from an earlier load.
func (rm *releaseManifest) loadChecksums(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	modTime := time.Time{}
	if info, statErr := f.Stat(); statErr == nil {
		modTime = info.ModTime()
	}

	parsed := make(map[string]string)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) != 2 {
			continue
		}

		hash := parts[0]
		filename := filepath.Base(parts[1])
		parsed[filename] = hash
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	rm.mu.Lock()
	rm.checksums = parsed
	rm.loadedModTime = modTime
	rm.mu.Unlock()

	return nil
}

// availablePlatforms returns platforms that have both a binary on disk
// and a matching checksum in the manifest.
func (rm *releaseManifest) availablePlatforms() []platformInfo {
	rm.refresh()
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	var available []platformInfo
	for _, p := range knownPlatforms {
		if _, ok := rm.checksums[p.Filename]; !ok {
			continue
		}
		path := filepath.Join(rm.releaseDir, p.Filename)
		if _, err := os.Stat(path); err != nil {
			continue
		}
		available = append(available, p)
	}

	return available
}

// verifyAndOpen checks the binary's SHA256 against the manifest, then
// returns an open file handle for serving. Returns an error if the hash
// doesn't match or the file doesn't exist.
func (rm *releaseManifest) verifyAndOpen(filename string) (*os.File, int64, error) {
	rm.refresh()
	rm.mu.Lock()
	expectedHash, ok := rm.checksums[filename]
	rm.mu.Unlock()

	if !ok {
		return nil, 0, fmt.Errorf("unknown release: %s", filename)
	}

	path := filepath.Join(rm.releaseDir, filename)

	// Compute hash
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("binary not found: %s", filename)
	}

	h := sha256.New()
	size, err := io.Copy(h, f)
	if err != nil {
		f.Close()
		return nil, 0, fmt.Errorf("failed to read binary: %w", err)
	}

	actualHash := hex.EncodeToString(h.Sum(nil))
	if actualHash != expectedHash {
		f.Close()
		return nil, 0, fmt.Errorf("integrity check failed for %s (expected %s vs calculated %s)", filename, expectedHash, actualHash)
	}

	// Seek back to the start to serve
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		f.Close()
		return nil, 0, fmt.Errorf("seek failed: %w", err)
	}

	return f, size, nil
}

func (rm *releaseManifest) get(filename string) (sha256 string, ok bool) {
	rm.refresh()
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	sha256, ok = rm.checksums[filename]
	return
}

func (rm *releaseManifest) expectedHash(goos, arch string) string {
	rm.refresh()
	filename := agentBinaryFilename(goos, arch)
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.checksums[filename]
}
