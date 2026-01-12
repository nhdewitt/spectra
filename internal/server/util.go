package server

import (
	"compress/gzip"
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

// getHostname ensures the hostname query param is present.
// Returns the hostname and true if found, or writes an error and returns false.
func getHostname(w http.ResponseWriter, r *http.Request) (string, bool) {
	hostname := r.URL.Query().Get("hostname")
	if hostname == "" {
		http.Error(w, "Missing hostname", http.StatusBadRequest)
		return "", false
	}

	return hostname, true
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
func (s *Server) queueHelper(w http.ResponseWriter, hostname string, cmdType protocol.CommandType, payload []byte, successMsg string) {
	cmd := protocol.Command{
		ID:      uuid.NewString(),
		Type:    cmdType,
		Payload: payload,
	}

	if s.Store.QueueCommand(hostname, cmd) {
		fmt.Fprintln(w, successMsg)
	} else {
		http.Error(w, "Queue full or agent not found", http.StatusServiceUnavailable)
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
