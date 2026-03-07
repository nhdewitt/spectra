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
)

// releaseManifest holds SHA256 checksums for pre-built agent binaries.
// Loaded from checksums.sha256 at startup, verified before serving downloads.
type releaseManifest struct {
	mu         sync.RWMutex
	checksums  map[string]string // filename -> sha256
	releaseDir string
}

// platformInfo describes a downloadable agent build.
type platformInfo struct {
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Variant  string `json:"variant,omitempty"` // "armv6", "armv7"
	Label    string `json:"label"`             // user-friendly: "Raspberry Pi ZZero/1 (armv6)"
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

	checksumPath := filepath.Join(releaseDir, "checksums.sha256")
	if err := rm.loadChecksums(checksumPath); err != nil {
		return rm
	}

	return rm
}

// loadChecksums parses a checksums.sha256 file in the format:
//
// <hex>  <filename>
func (rm *releaseManifest) loadChecksums(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	rm.mu.Lock()
	defer rm.mu.Unlock()

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
		rm.checksums[filename] = hash
	}

	return scanner.Err()
}

// availablePlatforms returns platforms that have both a binary on disk
// and a matching checksum in the manifest.
func (rm *releaseManifest) availablePlatforms() []platformInfo {
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
