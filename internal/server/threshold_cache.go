package server

import "context"

// thresholdValues holds the loaded thresholds in the plain float64/int32 shape
// the query params expect, converted once at load time so per-request reads need
// no conversion.
type thresholdValues struct {
	CPUWarn, CPUCrit   float64
	MemWarn, MemCrit   float64
	DiskWarn, DiskCrit float64
	TempWarn, TempCrit float64
	StaleSeconds       int32
	OfflineSeconds     int32
}

// loadThresholds reads the thresholds from the DB and caches them. Called on
// first use and after an update invalidates the cache. Returns the loaded values
// so the caller can use them immediately.
func (s *Server) loadThresholds(ctx context.Context) (*thresholdValues, error) {
	row, err := s.DB.GetStatusThresholds(ctx)
	if err != nil {
		return nil, err
	}

	tv := &thresholdValues{
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
	}
	s.thresholds.Store(tv)
	return tv, nil
}

// getThresholds returns the cached thresholds, loading them on first use. The
// common path is a lock-free atomic load; only the very first call or the call
// right after an invalidation hits the DB.
func (s *Server) getThresholds(ctx context.Context) (*thresholdValues, error) {
	if tv := s.thresholds.Load(); tv != nil {
		return tv, nil
	}
	return s.loadThresholds(ctx)
}

// invalidateThresholds clears the cache so the next read reloads from the DB.
// Called after a successful threshold update.
func (s *Server) invalidateThresholds() {
	s.thresholds.Store(nil)
}
