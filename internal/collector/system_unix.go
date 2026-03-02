//go:build linux || freebsd || darwin

package collector

import (
	"io"
	"strings"
)

// parseWhoFrom counts lines in the output of the `who` command.
func parseWhoFrom(r io.Reader) int {
	data, err := io.ReadAll(r)
	if err != nil {
		return 0
	}

	s := strings.TrimSpace(string(data))
	if s == "" {
		return 0
	}

	return len(strings.Split(s, "\n"))
}
