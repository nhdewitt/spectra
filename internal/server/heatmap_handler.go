package server

import (
	"net/http"
	"time"

	"github.com/nhdewitt/spectra/internal/database"
)

type Cell struct {
	Bucket time.Time `json:"bucket"`
	Value  float64   `json:"value"`
}

type AgentHeatmap struct {
	AgentID  string `json:"agent_id"`
	Hostname string `json:"hostname"`
	CPU      []Cell `json:"cpu"`
	Mem      []Cell `json:"mem"`
	Disk     []Cell `json:"disk"`
}

func (s *Server) handleFleetHeatmap(w http.ResponseWriter, r *http.Request) {
	startTime, endTime, err := parseTimeRange(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	rows, err := s.DB.GetFleetHeatmap(r.Context(), database.GetFleetHeatmapParams{
		StartTime: startTime,
		EndTime:   endTime,
	})
	if err != nil {
		s.dbError(w, err, "handleFleetHeatmap")
		return
	}

	agentMap := make(map[string]*AgentHeatmap)
	for _, row := range rows {
		idStr := formatUUID(row.AgentID)
		ah, ok := agentMap[idStr]
		if !ok {
			ah = &AgentHeatmap{
				AgentID: idStr,
				CPU:     []Cell{},
				Mem:     []Cell{},
				Disk:    []Cell{},
			}
			agentMap[idStr] = ah
		}
		cell := Cell{Bucket: row.Bucket.Time, Value: row.Value}
		switch row.Metric {
		case "cpu":
			ah.CPU = append(ah.CPU, cell)
		case "mem":
			ah.Mem = append(ah.Mem, cell)
		case "disk":
			ah.Disk = append(ah.Disk, cell)
		}
	}

	result := make([]AgentHeatmap, 0, len(agentMap))
	for _, ah := range agentMap {
		result = append(result, *ah)
	}

	respondJSON(w, http.StatusOK, result)
}
