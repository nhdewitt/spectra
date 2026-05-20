package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/nhdewitt/spectra/internal/protocol"
	"github.com/nhdewitt/spectra/internal/version"
)

func (a *Agent) selfUpdate(ctx context.Context, req protocol.UpdateAgentRequest) (*protocol.UpdateAgentResult, error) {
	if req.Version == version.Version {
		return &protocol.UpdateAgentResult{
			PreviousVersion: version.Version,
			NewVersion:      req.Version,
			Status:          "already_current",
		}, nil
	}

	a.Logger.Info("agent update starting",
		"current", version.Version,
		"target", req.Version)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, req.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("create download request: %w", err)
	}
	a.setHeaders(httpReq)
	httpReq.Header.Del("Content-Encoding")
	httpReq.Header.Del("Content-Type")

	resp, err := a.Client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download returned %d", resp.StatusCode)
	}

	currentBin, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("resolve executable path: %w", err)
	}
	currentBin, err = filepath.EvalSymlinks(currentBin)
	if err != nil {
		return nil, fmt.Errorf("resolve symlinks: %w", err)
	}

	dir := filepath.Dir(currentBin)
	tmp, err := os.CreateTemp(dir, "spectra-agent-update-*")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		// Clean up temp file if it still exists
		os.Remove(tmpPath)
	}()

	hasher := sha256.New()
	writer := io.MultiWriter(tmp, hasher)

	if _, err := io.Copy(writer, resp.Body); err != nil {
		tmp.Close()
		return nil, fmt.Errorf("download write failed: %w", err)
	}
	tmp.Close()

	// Verify SHA256
	gotHash := hex.EncodeToString(hasher.Sum(nil))
	if gotHash != req.SHA256 {
		return nil, fmt.Errorf("hash mismatch: expected %s, got %s", req.SHA256, gotHash)
	}

	a.Logger.Info("update binary verified", "sha256", gotHash)

	if runtime.GOOS != "windows" {
		if err := os.Chmod(tmpPath, 0755); err != nil {
			return nil, fmt.Errorf("chmod failed: %w", err)
		}
	}

	backupPath := currentBin + ".bak"
	os.Remove(backupPath) // clean up any previous backup

	if err := os.Rename(currentBin, backupPath); err != nil {
		return nil, fmt.Errorf("backup current binary: %w", err)
	}
	if err := os.Rename(tmpPath, currentBin); err != nil {
		os.Rename(backupPath, currentBin)
		return nil, fmt.Errorf("install new binary: %w", err)
	}

	a.Logger.Info("update installed, restarting",
		"previous", version.Version,
		"new", req.Version)

	result := &protocol.UpdateAgentResult{
		PreviousVersion: version.Version,
		NewVersion:      req.Version,
		Status:          "restarting",
	}

	// Exit after returning result - systemd/launchd/Windows service will restart
	go func() {
		<-ctx.Done()
		os.Exit(0)
	}()

	return result, nil
}
