package server

import (
	"encoding/hex"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nhdewitt/spectra/internal/database"
)

type agentOverview struct {
	ID               string   `json:"id"`
	Hostname         string   `json:"hostname"`
	OS               string   `json:"os"`
	Platform         string   `json:"platform"`
	Arch             string   `json:"arch"`
	CPUCores         int32    `json:"cpu_cores"`
	LastSeen         string   `json:"last_seen"`
	CPUUsage         *float64 `json:"cpu_usage"`
	LoadNormalized   *float64 `json:"load_normalized"`
	RAMPercent       *float64 `json:"ram_percent"`
	SwapPercent      *float64 `json:"swap_percent"`
	DiskMaxPercent   *float64 `json:"disk_max_percent"`
	NetRxBytes       *int64   `json:"net_rx_bytes"`
	NetTxBytes       *int64   `json:"net_tx_bytes"`
	MaxTemp          *float64 `json:"max_temp"`
	Uptime           *int64   `json:"uptime"`
	ProcessCount     *int32   `json:"process_count"`
	RebootRequired   bool     `json:"reboot_required"`
	MetricsUpdatedAt *string  `json:"metrics_updated_at"`
}

var uuidRegex = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// shortRanges maps quick range strings to durations.
var shortRanges = map[string]time.Duration{
	"5m":  5 * time.Minute,
	"15m": 15 * time.Minute,
	"1h":  1 * time.Hour,
	"6h":  6 * time.Hour,
	"24h": 24 * time.Hour,
	"7d":  7 * 24 * time.Hour,
	"30d": 30 * 24 * time.Hour,
}

// parseTimeRange extracts a time range from query parameters.
// Two modes:
//   - Quick range: ?range=1h
//   - Calendar range: ?start=YYYY-MM-DDT00:00:00Z&end=YYYY-MM-DDT00:00:00Z
//
// If end is omitted in calendar, it defaults to now.
// Start is clamped to the 30-day retention boundary.
func parseTimeRange(r *http.Request) (pgtype.Timestamptz, pgtype.Timestamptz, error) {
	now := time.Now()
	oldest := now.AddDate(0, 0, -30)

	if raw := r.URL.Query().Get("range"); raw != "" {
		d, ok := shortRanges[raw]
		if !ok {
			return pgtype.Timestamptz{}, pgtype.Timestamptz{}, fmt.Errorf("invalid range %q, valid: 5m, 15m, 1h, 6h, 24h, 7d, 30d", raw)
		}
		start := now.Add(-d)
		if start.Before(oldest) {
			start = oldest
		}
		return pgTimestamp(start), pgTimestamp(now), nil
	}

	startStr := r.URL.Query().Get("start")
	if startStr == "" {
		return pgTimestamp(now.Add(-1 * time.Hour)), pgTimestamp(now), nil
	}

	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		return pgtype.Timestamptz{}, pgtype.Timestamptz{}, fmt.Errorf("invalid start time, use RFC3339 format")
	}

	end := now
	if endStr := r.URL.Query().Get("end"); endStr != "" {
		end, err = time.Parse(time.RFC3339, endStr)
		if err != nil {
			return pgtype.Timestamptz{}, pgtype.Timestamptz{}, fmt.Errorf("invalid end time, use RFC3339 format")
		}
	}

	if start.Before(oldest) {
		start = oldest
	}
	if end.After(now) {
		end = now
	}
	if start.After(end) {
		return pgtype.Timestamptz{}, pgtype.Timestamptz{}, fmt.Errorf("start time must be before end time")
	}

	return pgTimestamp(start), pgTimestamp(end), nil
}

// parseAgentID extracts and validates the agent UUID from the path.
func parseAgentID(r *http.Request) (string, error) {
	id := r.PathValue("id")
	if !uuidRegex.MatchString(id) {
		return "", fmt.Errorf("invalid agent ID")
	}
	return id, nil
}

// handleOverview returns all agents with their current metrics for the dashboard.
func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	rows, err := s.DB.GetOverview(r.Context())
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	result := make([]agentOverview, 0, len(rows))
	for _, row := range rows {
		a := agentOverview{
			Hostname: row.Hostname,
			OS:       row.Os.String,
			CPUCores: row.CpuCores.Int32,
		}

		if row.ID.Valid {
			a.ID = formatUUID(row.ID)
		}
		if row.Arch.Valid {
			a.Arch = row.Arch.String
		}
		if row.LastSeen.Valid {
			a.LastSeen = row.LastSeen.Time.Format(time.RFC3339)
		}
		if row.UpdatedAt.Valid {
			ts := row.UpdatedAt.Time.Format(time.RFC3339)
			a.MetricsUpdatedAt = &ts
		}
		if row.CpuUsage.Valid {
			a.CPUUsage = &row.CpuUsage.Float64
		}
		if row.LoadNormalized.Valid {
			a.LoadNormalized = &row.LoadNormalized.Float64
		}
		if row.RamPercent.Valid {
			a.RAMPercent = &row.RamPercent.Float64
		}
		if row.SwapPercent.Valid {
			a.SwapPercent = &row.SwapPercent.Float64
		}
		if row.DiskMaxPercent.Valid {
			a.DiskMaxPercent = &row.DiskMaxPercent.Float64
		}
		if row.NetRxBytes.Valid {
			a.NetRxBytes = &row.NetRxBytes.Int64
		}
		if row.NetTxBytes.Valid {
			a.NetTxBytes = &row.NetTxBytes.Int64
		}
		if row.MaxTemp.Valid {
			a.MaxTemp = &row.MaxTemp.Float64
		}
		if row.Uptime.Valid {
			a.Uptime = &row.Uptime.Int64
		}
		if row.ProcessCount.Valid {
			a.ProcessCount = &row.ProcessCount.Int32
		}
		if row.RebootRequired.Valid {
			a.RebootRequired = row.RebootRequired.Bool
		}

		result = append(result, a)
	}

	respondJSON(w, http.StatusOK, result)
}

// handleGetAgent returns details for a single agent.
func (s *Server) handleGetAgent(w http.ResponseWriter, r *http.Request) {
	agentID, err := parseAgentID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	agent, err := s.DB.GetAgent(r.Context(), uuidParam(agentID))
	if err != nil {
		http.Error(w, "agent not found", http.StatusNotFound)
		return
	}

	respondJSON(w, http.StatusOK, agent)
}

// handleDeleteAgent removes an agent and all associated data.
func (s *Server) handleDeleteAgent(w http.ResponseWriter, r *http.Request) {
	agentID, err := parseAgentID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.DB.DeleteAgent(r.Context(), uuidParam(agentID)); err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	s.Store.Remove(agentID)
	w.WriteHeader(http.StatusNoContent)
}

// handleGetCPU returns CPU metrics for an agent over a time range.
func (s *Server) handleGetCPU(w http.ResponseWriter, r *http.Request) {
	uid, start, end, ok := s.parseRangeRequest(w, r)
	if !ok {
		return
	}
	rows, err := s.DB.GetCPURange(r.Context(), database.GetCPURangeParams{
		AgentID: uid, StartTime: start, EndTime: end,
	})
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, rows)
}

// handleGetMemory returns memory metrics for an agent over a time range.
func (s *Server) handleGetMemory(w http.ResponseWriter, r *http.Request) {
	uid, start, end, ok := s.parseRangeRequest(w, r)
	if !ok {
		return
	}
	rows, err := s.DB.GetMemoryRange(r.Context(), database.GetMemoryRangeParams{
		AgentID: uid, StartTime: start, EndTime: end,
	})
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, rows)
}

// handleGetDisk returns disk metrics for an agent over a time range.
func (s *Server) handleGetDisk(w http.ResponseWriter, r *http.Request) {
	uid, start, end, ok := s.parseRangeRequest(w, r)
	if !ok {
		return
	}
	rows, err := s.DB.GetDiskRange(r.Context(), database.GetDiskRangeParams{
		AgentID: uid, StartTime: start, EndTime: end,
	})
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, rows)
}

// handleGetDiskIO returns diskio metrics for an agent over a time range.
func (s *Server) handleGetDiskIO(w http.ResponseWriter, r *http.Request) {
	uid, start, end, ok := s.parseRangeRequest(w, r)
	if !ok {
		return
	}
	rows, err := s.DB.GetDiskIORange(r.Context(), database.GetDiskIORangeParams{
		AgentID: uid, StartTime: start, EndTime: end,
	})
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, rows)
}

// handleGetNetwork returns network metrics for an agent over a time range.
func (s *Server) handleGetNetwork(w http.ResponseWriter, r *http.Request) {
	uid, start, end, ok := s.parseRangeRequest(w, r)
	if !ok {
		return
	}
	rows, err := s.DB.GetNetworkRange(r.Context(), database.GetNetworkRangeParams{
		AgentID: uid, StartTime: start, EndTime: end,
	})
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, rows)
}

// handleGetTemperature returns temperature metrics for an agent over a time range.
func (s *Server) handleGetTemperature(w http.ResponseWriter, r *http.Request) {
	uid, start, end, ok := s.parseRangeRequest(w, r)
	if !ok {
		return
	}
	rows, err := s.DB.GetTemperatureRange(r.Context(), database.GetTemperatureRangeParams{
		AgentID: uid, StartTime: start, EndTime: end,
	})
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, rows)
}

// handleGetSystem returns system metrics for an agent over a time range.
func (s *Server) handleGetSystem(w http.ResponseWriter, r *http.Request) {
	uid, start, end, ok := s.parseRangeRequest(w, r)
	if !ok {
		return
	}
	rows, err := s.DB.GetSystemRange(r.Context(), database.GetSystemRangeParams{
		AgentID: uid, StartTime: start, EndTime: end,
	})
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, rows)
}

// handleGetContainers returns container metrics for an agent over a time range.
func (s *Server) handleGetContainers(w http.ResponseWriter, r *http.Request) {
	uid, start, end, ok := s.parseRangeRequest(w, r)
	if !ok {
		return
	}
	rows, err := s.DB.GetContainerRange(r.Context(), database.GetContainerRangeParams{
		AgentID: uid, StartTime: start, EndTime: end,
	})
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, rows)
}

// handleGetWifi returns WiFi metrics for an agent over a time range.
func (s *Server) handleGetWifi(w http.ResponseWriter, r *http.Request) {
	uid, start, end, ok := s.parseRangeRequest(w, r)
	if !ok {
		return
	}
	rows, err := s.DB.GetWifiRange(r.Context(), database.GetWifiRangeParams{
		AgentID: uid, StartTime: start, EndTime: end,
	})
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, rows)
}

// handleGetPi returns Raspberry Pi metrics for an agent over a time range.
func (s *Server) handleGetPi(w http.ResponseWriter, r *http.Request) {
	uid, start, end, ok := s.parseRangeRequest(w, r)
	if !ok {
		return
	}
	rows, err := s.DB.GetPiRange(r.Context(), database.GetPiRangeParams{
		AgentID: uid, StartTime: start, EndTime: end,
	})
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, rows)
}

// handleGetProcesses returns the top processes for an agent, sorted by CPU or memory.
func (s *Server) handleGetProcesses(w http.ResponseWriter, r *http.Request) {
	agentID, err := parseAgentID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	uid := uuidParam(agentID)
	limit := int32(20)

	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := fmt.Sscanf(l, "%d", &limit); n != 1 || err != nil {
			http.Error(w, "invalid limit", http.StatusBadRequest)
			return
		}
		if limit < 1 || limit > 100 {
			http.Error(w, "limit must be between 1 and 100", http.StatusBadRequest)
			return
		}
	}

	sort := r.URL.Query().Get("sort")
	ctx := r.Context()

	var rows []database.CurrentProcess
	switch sort {
	case "memory", "mem":
		rows, err = s.DB.GetProcessesByMemory(ctx, database.GetProcessesByMemoryParams{
			AgentID: uid, Limit: limit,
		})

	default:
		rows, err = s.DB.GetProcessesByCPU(ctx, database.GetProcessesByCPUParams{
			AgentID: uid, Limit: limit,
		})

	}

	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, rows)
}

// handleGetServices returns the current services for an agent.
func (s *Server) handleGetServices(w http.ResponseWriter, r *http.Request) {
	agentID, err := parseAgentID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	rows, err := s.DB.GetServices(r.Context(), uuidParam(agentID))
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, rows)
}

// handleGetApplications returns the installed applications for an agent.
func (s *Server) handleGetApplications(w http.ResponseWriter, r *http.Request) {
	agentID, err := parseAgentID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	rows, err := s.DB.GetApplications(r.Context(), uuidParam(agentID))
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, rows)
}

// handleGetUpdates returns the current update status for an agent.
func (s *Server) handleGetUpdates(w http.ResponseWriter, r *http.Request) {
	agentID, err := parseAgentID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	row, err := s.DB.GetUpdates(r.Context(), uuidParam(agentID))
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, row)
}

// handleListAgents returns the agents registered to the server.
func (s *Server) handleListAgents(w http.ResponseWriter, r *http.Request) {
	rows, err := s.DB.ListAgents(r.Context())
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, rows)
}

func (s *Server) parseRangeRequest(w http.ResponseWriter, r *http.Request) (pgtype.UUID, pgtype.Timestamptz, pgtype.Timestamptz, bool) {
	agentID, err := parseAgentID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return pgtype.UUID{}, pgtype.Timestamptz{}, pgtype.Timestamptz{}, false
	}
	start, end, err := parseTimeRange(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return pgtype.UUID{}, pgtype.Timestamptz{}, pgtype.Timestamptz{}, false
	}
	return uuidParam(agentID), start, end, true
}

func pgTimestamp(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func formatUUID(u pgtype.UUID) string {
	b := u.Bytes
	var buf [36]byte

	hex.Encode(buf[0:8], b[0:4])
	buf[8] = '-'
	hex.Encode(buf[9:13], b[4:6])
	buf[13] = '-'
	hex.Encode(buf[14:18], b[6:8])
	buf[18] = '-'
	hex.Encode(buf[19:23], b[8:10])
	buf[23] = '-'
	hex.Encode(buf[24:36], b[10:16])

	return string(buf[:])
}
