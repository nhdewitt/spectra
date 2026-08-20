package server

import (
	"fmt"
	"net/http"

	"github.com/nhdewitt/spectra/internal/database"
)

// thresholdView is the JSON shape for the global status thresholds, sent to and
// accepted from the dashboard. Field names mirror the frontend Thresholds type.
type thresholdView struct {
	CPUWarn        float64 `json:"cpu_warn"`
	CPUCrit        float64 `json:"cpu_crit"`
	MemWarn        float64 `json:"mem_warn"`
	MemCrit        float64 `json:"mem_crit"`
	DiskWarn       float64 `json:"disk_warn"`
	DiskCrit       float64 `json:"disk_crit"`
	TempWarn       float64 `json:"temp_warn"`
	TempCrit       float64 `json:"temp_crit"`
	StaleSeconds   int32   `json:"stale_seconds"`
	OfflineSeconds int32   `json:"offline_seconds"`
}

// handleGetThresholds returns the global status thresholds. Read access is open
// to any authenticated user since the dashboard needs them to classify agents;
// writes are admin-gated in handleUpdateThresholds.
//
// GET /api/v1/thresholds
func (s *Server) handleGetThresholds(w http.ResponseWriter, r *http.Request) {
	row, err := s.DB.GetStatusThresholds(r.Context())
	if err != nil {
		s.dbError(w, err, "handleGetThresholds")
		return
	}

	respondJSON(w, http.StatusOK, thresholdView{
		CPUWarn:        row.CpuWarn,
		CPUCrit:        row.CpuCrit,
		MemWarn:        row.MemWarn,
		MemCrit:        row.MemCrit,
		DiskWarn:       row.DiskWarn,
		DiskCrit:       row.DiskCrit,
		TempWarn:       row.TempWarn,
		TempCrit:       row.TempCrit,
		StaleSeconds:   row.StaleSeconds,
		OfflineSeconds: row.OfflineSeconds,
	})
}

// handleUpdateThresholds replaces the global status thresholds. Admin-only.
//
// PUT /api/v1/admin/thresholds
func (s *Server) handleUpdateThresholds(w http.ResponseWriter, r *http.Request) {
	var in thresholdView
	if err := decodeJSONBody(r, &in, maxStandardBody); err != nil {
		http.Error(w, err.Error(), badBodyStatus(err))
		return
	}

	if err := validateThresholds(in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err := s.DB.UpsertStatusThresholds(r.Context(), database.UpsertStatusThresholdsParams{
		CpuWarn:        in.CPUWarn,
		CpuCrit:        in.CPUCrit,
		MemWarn:        in.MemWarn,
		MemCrit:        in.MemCrit,
		DiskWarn:       in.DiskWarn,
		DiskCrit:       in.DiskCrit,
		TempWarn:       in.TempWarn,
		TempCrit:       in.TempCrit,
		StaleSeconds:   in.StaleSeconds,
		OfflineSeconds: in.OfflineSeconds,
	})
	if err != nil {
		s.dbError(w, err, "handleUpdateThresholds")
		return
	}
	s.invalidateThresholds()

	respondJSON(w, http.StatusOK, in)
}

// validateThresholds enforces sane bounds: percentages in [0,100] with warn
// not above crit, and positive, ordered staleness windows.
func validateThresholds(t thresholdView) error {
	pairs := []struct {
		name       string
		warn, crit float64
	}{
		{"cpu", t.CPUWarn, t.CPUCrit},
		{"mem", t.MemWarn, t.MemCrit},
		{"disk", t.DiskWarn, t.DiskCrit},
		{"temp", t.TempWarn, t.TempCrit},
	}
	for _, p := range pairs {
		if p.warn < 0 || p.crit < 0 {
			return fmt.Errorf("%s thresholds must be non-negative", p.name)
		}
		if p.name != "temp" && (p.warn > 100 || p.crit > 100) {
			return fmt.Errorf("%s thresholds must be <= 100", p.name)
		}
		if p.warn > p.crit {
			return fmt.Errorf("%s warn (%.0f) must not exceed crit (%.0f)", p.name, p.warn, p.crit)
		}
	}
	if t.StaleSeconds <= 0 || t.OfflineSeconds <= 0 {
		return fmt.Errorf("stale and offline windows must be positive")
	}
	if t.StaleSeconds >= t.OfflineSeconds {
		return fmt.Errorf("stale window (%ds) must be shorter than offline window (%ds)", t.StaleSeconds, t.OfflineSeconds)
	}
	return nil
}
