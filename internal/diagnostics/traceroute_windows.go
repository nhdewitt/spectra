//go:build windows

package diagnostics

import (
	"context"
	"fmt"
	"os/exec"
)

func runTraceroute(ctx context.Context, target string) (string, error) {
	if target == "" {
		return "", fmt.Errorf("target required")
	}

	cmd := "tracert"
	args := []string{
		"-d",
		"-w", "2000",
		"-4",
		target,
	}

	out, err := exec.CommandContext(ctx, cmd, args...).CombinedOutput()
	return string(out), err
}
