//go:build !windows

package main

import "github.com/nhdewitt/spectra/internal/agent"

func runService(_ *agent.Agent) error {
	return nil
}

func isWindowsService() bool {
	return false
}
