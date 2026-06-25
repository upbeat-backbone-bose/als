package testutil

import (
	"testing"
	"time"
)

func TestWaitForConditionTrue(t *testing.T) {
	t.Parallel()

	start := time.Now()
	calls := 0
	WaitFor(t, time.Second, "first call should succeed", func() bool {
		calls++
		return true
	})
	if calls != 1 {
		t.Errorf("cond called %d times; want 1", calls)
	}
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Errorf("WaitFor took %v; want near-instant return when cond is true on first call", elapsed)
	}
}

func TestWaitForPollingUntilTrue(t *testing.T) {
	t.Parallel()

	calls := 0
	WaitFor(t, time.Second, "should return after a few polls", func() bool {
		calls++
		return calls >= 3
	})
	if calls != 3 {
		t.Errorf("cond called %d times; want 3", calls)
	}
}
