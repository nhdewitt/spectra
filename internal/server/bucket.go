package server

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// bucketInterval returns the TimescaleDB time_bucket interval string
// for the given time range, or empty string if raw data should be used.
//
// Thresholds:
//   - <= 1h: no bucketing
//   - <= 6h: 1 minute (~360 rows max)
//   - <= 24h: 5 minutes (~288 rows max)
//   - <= 7d: 15 minutes (~672 rows max)
//   - <= 30d: 1 hour (~720 rows max)
func bucketInterval(start, end pgtype.Timestamptz) string {
	dur := end.Time.Sub(start.Time)
	switch {
	case dur <= time.Hour:
		return ""
	case dur <= 6*time.Hour:
		return "1 minute"
	case dur <= 24*time.Hour:
		return "5 minutes"
	case dur <= 7*24*time.Hour:
		return "15 minutes"
	default:
		return "1 hour"
	}
}
