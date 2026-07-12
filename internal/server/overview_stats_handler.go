package server

import (
	"net/http"

	"github.com/nhdewitt/spectra/internal/database"
)

// overviewStatsView is the StatBar's fleet-wide counts.
type overviewStatsView struct {
	Total   int64 `json:"total"`
	Online  int64 `json:"online"`
	Warn    int64 `json:"warn"`
	Crit    int64 `json:"crit"`
	Stale   int64 `json:"stale"`
	Offline int64 `json:"offline"`
	Reboot  int64 `json:"reboot"`
}

// handleOverviewStats returns fleet-wide status counts for the dashboard
// StatBar. Because the paginated overview returns only one page, these totals
// are computed server-side rather than derived client-side.
//
// GET /api/v1/overview/stats
func (s *Server) handleOverviewStats(w http.ResponseWriter, r *http.Request) {
	tv, err := s.getThresholds(r.Context())
	if err != nil {
		s.dbError(w, err, "handleOverviewStats")
		return
	}

	st, err := s.DB.GetOverviewStats(r.Context(), database.GetOverviewStatsParams{
		CPUWarn:             tv.CPUWarn,
		CPUCrit:             tv.CPUCrit,
		MemWarn:             tv.MemWarn,
		MemCrit:             tv.MemCrit,
		DiskWarn:            tv.DiskWarn,
		DiskCrit:            tv.DiskCrit,
		TempWarn:            tv.TempWarn,
		TempCrit:            tv.TempCrit,
		StaleAfterSeconds:   tv.StaleSeconds,
		OfflineAfterSeconds: tv.OfflineSeconds,
	})
	if err != nil {
		s.dbError(w, err, "handleOverviewStats")
		return
	}

	respondJSON(w, http.StatusOK, overviewStatsView{
		Total:   st.Total,
		Online:  st.Online,
		Warn:    st.Warn,
		Crit:    st.Crit,
		Stale:   st.Stale,
		Offline: st.Offline,
		Reboot:  st.Reboot,
	})
}
