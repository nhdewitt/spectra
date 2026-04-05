package server

import (
	"net/http"
	"time"

	"github.com/nhdewitt/spectra/internal/database"
)

// sparklineData holds the sparkline arrays for a single agent.
type sparklineData struct {
	CPU  []float64 `json:"cpu"`
	Mem  []float64 `json:"mem"`
	Disk []float64 `json:"disk"`
}

type Point struct {
	T time.Time `json:"t"`
	V float64   `json:"v"`
}

type AgentChart struct {
	CPU  []Point `json:"cpu"`
	Mem  []Point `json:"mem"`
	Disk []Point `json:"disk"`
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

func (s *Server) handleFleetChart(w http.ResponseWriter, r *http.Request) {
	startTime, endTime, err := parseTimeRange(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	bucket := bucketInterval(startTime, endTime)
	if bucket == "" {
		bucket = "30 seconds"
	}

	ctx := r.Context()

	result := make(map[string]*AgentChart)
	ensure := func(id string) *AgentChart {
		if c, ok := result[id]; ok {
			return c
		}
		c := &AgentChart{CPU: []Point{}, Mem: []Point{}, Disk: []Point{}}
		result[id] = c
		return c
	}

	cpuRows, err := s.DB.GetFleetSparkCPU(ctx, database.GetFleetSparkCPUParams{
		BucketInterval: bucket,
		StartTime:      startTime,
		EndTime:        endTime,
	})
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	for _, row := range cpuRows {
		id := formatUUID(row.AgentID)
		c := ensure(id)
		c.CPU = append(c.CPU, Point{T: row.Bucket.Time, V: row.Usage})
	}

	memRows, err := s.DB.GetFleetSparkMemory(ctx, database.GetFleetSparkMemoryParams{
		BucketInterval: bucket,
		StartTime:      startTime,
		EndTime:        endTime,
	})
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	for _, row := range memRows {
		id := formatUUID(row.AgentID)
		c := ensure(id)
		c.Mem = append(c.Mem, Point{T: row.Bucket.Time, V: row.RamPercent})
	}

	diskRows, err := s.DB.GetFleetSparkDisk(ctx, database.GetFleetSparkDiskParams{
		BucketInterval: bucket,
		StartTime:      startTime,
		EndTime:        endTime,
	})
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	for _, row := range diskRows {
		id := formatUUID(row.AgentID)
		c := ensure(id)
		c.Disk = append(c.Disk, Point{T: row.Bucket.Time, V: row.MaxPercent})
	}

	respondJSON(w, http.StatusOK, result)
}
