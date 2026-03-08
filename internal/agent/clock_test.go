package agent

import (
	"context"
	"errors"
	"testing"
	"time"
)

func mockTime(tb testing.TB) *time.Time {
	tb.Helper()
	fakeTime := time.Now()
	nowFunc = func() time.Time { return fakeTime }
	tb.Cleanup(func() { nowFunc = time.Now })
	return &fakeTime
}

func TestWaitForNextMinute_Aligns(t *testing.T) {
	fakeTime := mockTime(t)
	// Set to 5ms before the minute
	*fakeTime = time.Now().Truncate(time.Minute).Add(time.Minute).Add(-5 * time.Millisecond)

	err := waitForNextMinute(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWaitForNextMinute_ContextCancel(t *testing.T) {
	fakeTime := mockTime(t)
	*fakeTime = time.Now().Truncate(time.Minute).Add(30 * time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := waitForNextMinute(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}
