package server

import (
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nhdewitt/spectra/internal/protocol"
)

var uuidRegex = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

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
		if err := json.NewEncoder(w).Encode(data); err != nil {
			log.Printf("Failed to write JSON response: %v", err)
		}
	}
}

// respondError sends a JSON error response.
func respondError(w http.ResponseWriter, status int, msg string) {
	respondJSON(w, status, map[string]string{"error": msg})
}

// queueHelper abstracts the repetitive command creation/queueing logic for Admin handlers.
func (s *Server) queueHelper(w http.ResponseWriter, agentID string, cmdType protocol.CommandType, payload []byte, successMsg string) {
	cmd := protocol.Command{
		ID:      uuid.NewString(),
		Type:    cmdType,
		Payload: payload,
	}

	err := s.CmdQueue.Send(agentID, cmd)
	if err != nil {
		s.Logger.Error("queue full or agent not found", "error", err, "handler", "queueHelper")
		http.Error(w, "Queue full or agent not found", http.StatusServiceUnavailable)
		return
	}

	s.Commands.Track(cmd.ID, cmdType, agentID)
	s.Logger.Info("command queued", "agent_id", agentID, "command", cmdType)
	respondJSON(w, http.StatusAccepted, map[string]string{
		"command_id": cmd.ID,
		"message":    successMsg,
	})
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
		s.Logger.Warn("no agent ID provided", "handler", "getTargetAgent")
		http.Error(w, "agent ID required", http.StatusBadRequest)
		return "", false
	}

	var uid pgtype.UUID
	if err := uid.Scan(agentID); err != nil {
		s.Logger.Warn("invalid agent ID", "agent_id", agentID, "handler", "getTargetAgent")
		http.Error(w, "invalid agent ID", http.StatusBadRequest)
		return "", false
	}

	_, err := s.DB.GetAgent(r.Context(), uid)
	if err != nil {
		s.Logger.Warn("agent not found", "agent_id", agentID, "handler", "getTargetAgent")
		http.Error(w, "agent not found", http.StatusNotFound)
		return "", false
	}

	return agentID, true
}

// fleetQuery runs a sql query and groups the results into a map by agent ID.
func fleetQuery[P any, R any](ctx context.Context, queryFn func(context.Context, P) ([]R, error), params P, extract func(R) (string, FleetChartPoint)) (map[string][]FleetChartPoint, error) {
	rows, err := queryFn(ctx, params)
	if err != nil {
		return nil, err
	}
	result := make(map[string][]FleetChartPoint)
	for _, row := range rows {
		id, pt := extract(row)
		result[id] = append(result[id], pt)
	}
	return result, nil
}

func (s *Server) dbError(w http.ResponseWriter, err error, handler string) {
	s.Logger.Error("database query failed", "error", err, "handler", handler)
	http.Error(w, "database error", http.StatusInternalServerError)
}

func isPgUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func mustMarshal(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte("{}")
	}
	return b
}

// parsePathID extracts and validates the agent UUID from the path.
func parsePathID(r *http.Request) (string, error) {
	id := r.PathValue("id")
	if !uuidRegex.MatchString(id) {
		return "", fmt.Errorf("invalid agent ID")
	}
	return id, nil
}
