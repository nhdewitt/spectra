package setup

import (
	"fmt"
	"os/exec"
)

// hasSystemd reports whether systemctl is available.
func hasSystemd() bool {
	_, err := exec.LookPath("systemctl")
	return err == nil
}

// StartService reloads systemd and enables + (re)starts spectra-server. The unit
// file itself is installed by the deploy layer (`make setup`), not here - setup
// only brings the configured service up once the DB, admin, config, and key are
// in place. Returns (started, error): started is false when systemd is absent.
//
// restart (not just start) ensures a rerun picks up a new binary or config even
// when the service was already running.
func StartService() (bool, error) {
	if !hasSystemd() {
		return false, nil
	}

	steps := [][]string{
		{"systemctl", "daemon-reload"},
		{"systemctl", "enable", "spectra-server"},
		{"systemctl", "restart", "spectra-server"},
	}

	for _, step := range steps {
		if out, err := exec.Command(step[0], step[1:]...).CombinedOutput(); err != nil {
			return false, fmt.Errorf("%s: %w: %s", step[1], err, out)
		}
	}

	return true, nil
}
