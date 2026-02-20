package server

import (
	"compress/gzip"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/nhdewitt/spectra/internal/protocol"
)

// decodeJSONBody reads the request body, handling optional gzip compression,
// and decodes it into the provided target struct.
func decodeJSONBody(r *http.Request, target any) error {
	var reader io.ReadCloser = r.Body

	if r.Header.Get("Content-Encoding") == "gzip" {
		gz, err := gzip.NewReader(r.Body)
		if err != nil {
			return fmt.Errorf("bad gzip body: %w", err)
		}
		reader = gz
	}
	defer reader.Close()

	if err := json.NewDecoder(reader).Decode(target); err != nil {
		return fmt.Errorf("invalid json: %w", err)
	}

	return nil
}

func getAgentID(r *http.Request) string {
	return r.Header.Get("X-Agent-ID")
}

// respondJSON sends a JSON response with the given status code.
func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}

// queueHelper abstracts the repetitive command creation/queueing logic for Admin handlers.
func (s *Server) queueHelper(w http.ResponseWriter, agentID string, cmdType protocol.CommandType, payload []byte, successMsg string) {
	cmd := protocol.Command{
		ID:      uuid.NewString(),
		Type:    cmdType,
		Payload: payload,
	}

	err := s.Store.QueueCommand(agentID, cmd)
	if err != nil {
		http.Error(w, "Queue full or agent not found", http.StatusServiceUnavailable)
	} else {
		fmt.Fprintln(w, successMsg)
	}
}

func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func generateSecret(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *Server) getTargetAgent(w http.ResponseWriter, r *http.Request) (string, bool) {
	agentID := r.URL.Query().Get("agent")
	if agentID == "" {
		http.Error(w, "agent ID required", http.StatusBadRequest)
		return "", false
	}
	if !s.Store.Exists(agentID) {
		http.Error(w, "agent not registered", http.StatusNotFound)
		return "", false
	}
	return agentID, true
}
