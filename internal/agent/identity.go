package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type AgentIdentity struct {
	ID     string `json:"id"` // UUID
	Secret string `json:"secret"`
}

func loadIdentity() (AgentIdentity, error) {
	data, err := os.ReadFile(identityPath())
	if err != nil {
		return AgentIdentity{}, err
	}

	var id AgentIdentity
	if err := json.Unmarshal(data, &id); err != nil {
		return AgentIdentity{}, err
	}
	if id.ID == "" || id.Secret == "" {
		return AgentIdentity{}, fmt.Errorf("identity file missing id or secret")
	}

	return id, nil
}

func saveIdentity(id AgentIdentity) error {
	dir := filepath.Dir(identityPath())
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating identity dir: %w", err)
	}
	data, err := json.Marshal(id)
	if err != nil {
		return err
	}
	return os.WriteFile(identityPath(), data, 0600)
}
