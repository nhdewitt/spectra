//go:build !windows

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

	cmd := "traceroute"
	args := []string{
		"-n",
		"-w", "2",
		"-q", "1",
		"-I",
		target,
	}

	out, err := exec.CommandContext(ctx, cmd, args...).CombinedOutput()
	return string(out), err
}
