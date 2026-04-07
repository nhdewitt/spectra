package server

import (
	"net/http"
	"time"

	"github.com/nhdewitt/spectra/internal/database"
)

// FleetChartPoint is the JSON shape returned for each data point.
type FleetChartPoint struct {
	T string  `json:"t"`
	V float64 `json:"v"`
}

// handleFleetChart returns the time-bucketed metric data for all agents.
//
// GET /api/v1/overview/fleet/chart
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

	metric := r.URL.Query().Get("metric")
	if metric == "" {
		metric = "cpu"
	}

	ctx := r.Context()
	var result map[string][]FleetChartPoint

	switch metric {
	case "cpu":
		result, err = fleetQuery(ctx, s.DB.GetFleetSparkCPU, database.GetFleetSparkCPUParams{
			BucketInterval: bucket, StartTime: startTime, EndTime: endTime,
		}, func(r database.GetFleetSparkCPURow) (string, FleetChartPoint) {
			return formatUUID(r.AgentID), FleetChartPoint{
				T: r.Bucket.Time.Format(time.RFC3339),
				V: r.Usage,
			}
		})

	case "mem":
		result, err = fleetQuery(ctx, s.DB.GetFleetSparkMemory, database.GetFleetSparkMemoryParams{
			BucketInterval: bucket, StartTime: startTime, EndTime: endTime,
		}, func(r database.GetFleetSparkMemoryRow) (string, FleetChartPoint) {
			return formatUUID(r.AgentID), FleetChartPoint{
				T: r.Bucket.Time.Format(time.RFC3339),
				V: r.RamPercent,
			}
		})

	case "disk":
		result, err = fleetQuery(ctx, s.DB.GetFleetSparkDisk, database.GetFleetSparkDiskParams{
			BucketInterval: bucket, StartTime: startTime, EndTime: endTime,
		}, func(r database.GetFleetSparkDiskRow) (string, FleetChartPoint) {
			return formatUUID(r.AgentID), FleetChartPoint{
				T: r.Bucket.Time.Format(time.RFC3339),
				V: r.MaxPercent,
			}
		})

	default:
		http.Error(w, "invalid metric", http.StatusBadRequest)
		return
	}

	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, result)
}
