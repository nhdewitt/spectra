package agent

import (
	"context"
	"fmt"
	"time"
)

// scheduleNightly runs the provided function every day at the specified hour/minute.
// It accounts for restarts and date rollovers.
func scheduleNightly(ctx context.Context, hour, minute int, fn func(context.Context)) {
	for {
		now := time.Now()
		next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())

		if now.After(next) {
			next = next.AddDate(0, 0, 1)
		}

		t := time.NewTimer(time.Until(next))

		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C:
			fn(ctx)
		}
	}
}

func formatBytes(b int) string {
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
