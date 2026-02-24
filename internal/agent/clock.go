package agent

import (
	"context"
	"time"
)

var nowFunc = time.Now

// waitForNextMinute blocks until the next minute boundary to align metric
// collection across all agents.
func waitForNextMinute(ctx context.Context) error {
	now := nowFunc()
	next := now.Truncate(time.Minute).Add(time.Minute)
	wait := next.Sub(now)

	select {
	case <-time.After(wait):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
