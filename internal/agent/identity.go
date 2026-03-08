package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Identity struct {
	ID     string `json:"id"` // UUID
	Secret string `json:"secret"`
}

func loadIdentity(path string) (Identity, error) {
	if path == "" {
		path = identityPath()
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Identity{}, err
	}

	var id Identity
	if err := json.Unmarshal(data, &id); err != nil {
		return Identity{}, err
	}
	if id.ID == "" || id.Secret == "" {
		return Identity{}, fmt.Errorf("identity file missing id or secret")
	}

	return id, nil
}

func saveIdentity(id Identity, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating identity dir: %w", err)
	}
	data, err := json.Marshal(id)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
