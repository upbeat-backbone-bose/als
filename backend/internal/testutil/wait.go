// Package testutil provides shared test helpers used across the
// backend. Anything in this package must be safe to import from
// any internal test (_test.go file) and must not be part of the
// production build.
package testutil

import (
	"runtime"
	"testing"
	"time"
)

// WaitFor polls cond once per millisecond, yielding to the
// scheduler between checks, until cond returns true or timeout
// elapses. On timeout it calls t.Fatalf -- callers therefore do
// not need to check for an error.
func WaitFor(t *testing.T, timeout time.Duration, msg string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for {
		if cond() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("WaitFor timed out after %v: %s", timeout, msg)
		}
		select {
		case <-tick.C:
		default:
			runtime.Gosched()
		}
	}
}
