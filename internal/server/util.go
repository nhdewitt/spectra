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

// Request body size classes, in decompressed bytes. Every decodeJSONBody call
// picks one, so adding an endpoint forces a deliberate choice rather than
// inheriting a default.
const (
	// maxAuthBody covers login and registration - username/password
	// These parse before any meaningful authentication, so they get
	// the tightest ceiling.
	maxAuthBody = 4 << 10
	// maxStandardBody covers ordinary admin and config payloads.
	maxStandardBody = 64 << 10
	// maxCommandResultBody covers diagnostic output. A verbose FETCH_LOGS
	// result from a chatty host runs to hundreds of KB, and squeezing it
	// into the standard limit would break diagnostics exactly when they
	// are needed.
	maxCommandResultBody = 4 << 20
	// maxMetricsBody covers a metric batch. A full maxUploadChunk drain is
	// roughly 2MB decompressed, so thise leaves 8x headroom. It must stay
	// above what an agent can produce in one request: the agent retries
	// any rejected batch, so a limit below that wedges it permanently.
	maxMetricsBody = 16 << 20
)

// errBodyTooLarge is returned when a request body exceeds its size class.
// Distinct from a decode error so handlers can answer 413 instead of 400, and
// so the agent can tell "this will never be accepted" from "this is malformed".
var errBodyTooLarge = errors.New("request body too large")

// limitedReader is io.LimitReader with a distinguishable error.
//
// io.LimitReader reports EOF at the limit, which json.Decoder surfaces as an
// unexpected EOF, indistinguishable from a genuinely truncated body, so an
// oversized request would be answered as malfored JSON.
type limitedReader struct {
	r         io.Reader
	remaining int64
}

func (l *limitedReader) Read(p []byte) (int, error) {
	if l.remaining <= 0 {
		return 0, errBodyTooLarge
	}
	if int64(len(p)) > l.remaining {
		p = p[:l.remaining]
	}
	n, err := l.r.Read(p)
	l.remaining -= int64(n)
	return n, err
}

// decodeJSONBody reads the request body, handling optional gzip compression,
// and decodes it into the provided target struct.
//
// maxBytes bounds the decompressed stream. Bounding Content-Length alone is
// not enough; a small gzip body can expand by orders of magnitude, and this
// decoder previously fed the gzip reader straight into json.Decoder with
// nothing in between. The compressed side is bounded too, at the same figure,
// so an endless body is cut off before it is ever inflated.
func decodeJSONBody(r *http.Request, target any, maxBytes int64) error {
	var reader io.ReadCloser = http.MaxBytesReader(nil, r.Body, maxBytes)

	if r.Header.Get("Content-Encoding") == "gzip" {
		gz, err := gzip.NewReader(reader)
		if err != nil {
			return fmt.Errorf("bad gzip body: %w", err)
		}
		reader = gz
	}
	defer reader.Close()

	limited := &limitedReader{r: reader, remaining: maxBytes}

	if err := json.NewDecoder(limited).Decode(target); err != nil {
		if errors.Is(err, errBodyTooLarge) {
			return errBodyTooLarge
		}
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return errBodyTooLarge
		}
		return fmt.Errorf("invalid json: %w", err)
	}

	return nil
}

// badBodyStatus maps a decodeJSONBody error to a status code, so an oversized
// body is reported as 413 rather than being lumped in with malformed JSON.
func badBodyStatus(err error) int {
	if errors.Is(err, errBodyTooLarge) {
		return http.StatusRequestEntityTooLarge
	}
	return http.StatusBadRequest
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
	if errors.Is(err, context.Canceled) {
		return
	}
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

// parsePathID extracts and validates the UUID from the path.
func parsePathID(r *http.Request) (string, error) {
	id := r.PathValue("id")
	if !uuidRegex.MatchString(id) {
		return "", fmt.Errorf("invalid ID")
	}
	return id, nil
}
