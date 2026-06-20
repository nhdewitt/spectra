package setup

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nhdewitt/spectra/internal/fileutil"
)

// DefaultSecretEnvPath is the systemd EnvironmentFile holding SPECTRA_SECRET_KEY.
const DefaultSecretEnvPath = "/etc/spectra/spectra.env"

// secretKeyEnvVar must match secret.KeyEnvVar. Duplicated here to avoid a dependency
// for one constant; a test asserts they stay in sync.
const secretKeyEnvVar = "SPECTRA_SECRET_KEY"

// GenerateSecretKeyFile writes a new base64-encoded 32-byte key to an
// EnvironmentFile, unless one already exists. Returns (created, error) - created is
// false when the file was already present and left untouched, making reruns of setup
// idempotent.
func GenerateSecretKeyFile(path string) (bool, error) {
	if _, err := os.Stat(path); err == nil {
		return false, nil
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("stat secret env file: %w", err)
	}

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return false, fmt.Errorf("generate secret key: %w", err)
	}
	encoded := base64.StdEncoding.EncodeToString(key)

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return false, fmt.Errorf("creating config dir: %w", err)
	}

	line := fmt.Sprintf("%s=%s\n", secretKeyEnvVar, encoded)
	if err := fileutil.WriteSecure(path, []byte(line)); err != nil {
		return false, fmt.Errorf("writing secret env file: %w", err)
	}

	return true, nil
}
