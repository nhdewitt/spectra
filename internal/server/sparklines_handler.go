package server

import "net/http"

// sparklineData holds the sparkline arrays for a single agent.
type sparklineData struct {
	CPU  []float64 `json:"cpu"`
	Mem  []float64 `json:"mem"`
	Disk []float64 `json:"disk"`
}

// handleGetSparklines returns recent metric history for all agents in a single
// response. This allows sparklines to populate without requiring N*3 API calls
// that would trigger rate limits.
//
// GET /api/v1/overview/sparklines
//
// Response: map[agentID] -> { cpu: [...], mem: [...], disk: [...] }
func (s *Server) handleGetSparklines(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	result := make(map[string]*sparklineData)

	// ensure every agent entry exists
	ensure := func(agentID string) *sparklineData {
		if d, ok := result[agentID]; ok {
			return d
		}
		d := &sparklineData{
			CPU:  []float64{},
			Mem:  []float64{},
			Disk: []float64{},
		}
		result[agentID] = d
		return d
	}

	// CPU
	cpuRows, err := s.DB.GetRecentCPU(ctx)
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	for _, row := range cpuRows {
		id := formatUUID(row.AgentID)
		d := ensure(id)
		d.CPU = append(d.CPU, row.Usage.Float64)
	}

	// Memory
	memRows, err := s.DB.GetRecentMemory(ctx)
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	for _, row := range memRows {
		id := formatUUID(row.AgentID)
		d := ensure(id)
		d.Mem = append(d.Mem, row.RamPercent.Float64)
	}

	// Disk
	diskRows, err := s.DB.GetRecentDiskMax(ctx)
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	for _, row := range diskRows {
		id := formatUUID(row.AgentID)
		d := ensure(id)
		d.Disk = append(d.Disk, row.MaxPercent)
	}

	respondJSON(w, http.StatusOK, result)
}
