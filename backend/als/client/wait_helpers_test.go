package client

import (
	"runtime"
	"testing"
	"time"
)

// waitFor spins until cond returns true or timeout elapses. Replaces
// the legacy `time.Sleep(time.Millisecond)` polling pattern with a
// 1ms ticker select that gives the scheduler a real preemption
// window between polls.
//
// Why not pure runtime.Gosched: under -race, Gosched returns
// immediately and the test goroutine can burn a full CPU on
// cond(), starving other goroutines that need to make progress
// (e.g. the queue handler goroutine that the test is waiting on).
// A 1ms select gives the scheduler time to run everyone.
//
// cond must be cheap and idempotent. It is called at least once,
// possibly many times.
//
// linter's per-file analysis can't see across files.
//
//nolint:unparam // each call site picks a different timeout; the
func waitFor(t *testing.T, timeout time.Duration, msg string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for {
		if cond() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("waitFor timed out after %v: %s", timeout, msg)
		}
		select {
		case <-tick.C:
		default:
			runtime.Gosched()
		}
	}
}
