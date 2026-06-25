package client

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/samlm0/als/v2/internal/testutil"
)

// resetQueueForTest cancels any entries still parked in the queue, clears
// global state, and drains pending wakeup signals so each test starts
// from a known-clean baseline.
func resetQueueForTest(t *testing.T) {
	t.Helper()

	queueLock.Lock()
	for _, e := range queueEntries {
		if e != nil && e.cancel != nil {
			e.cancel()
		}
	}
	queueEntries = nil
	queueLock.Unlock()

	for {
		select {
		case <-queueWakeup:
		default:
			return
		}
	}
}

// startHandler launches an isolated HandleQueue goroutine bound to a
// cancellable context. The goroutine exits when the returned stop
// function is called, so tests can run independently without sharing a
// long-lived handler.
//
// The handler is guaranteed to be parked in its outer select on
// <-queueWakeup before startHandler returns.
func startHandler(t *testing.T) (stop func()) {
	t.Helper()
	handlerCtx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		HandleQueue(handlerCtx)
	}()

	// Synchronization barrier: send two warm-up wakeups. The first send
	// blocks until the handler is in its outer select; the handler
	// consumes it, runs an empty inner pass, then returns to the outer
	// select. The second send then rendezvous with the parked handler,
	// guaranteeing by the time it returns that the handler is ready.
	doneWarmup := make(chan struct{})
	go func() {
		defer close(doneWarmup)
		queueWakeup <- struct{}{}
		queueWakeup <- struct{}{}
	}()
	select {
	case <-doneWarmup:
	case <-time.After(2 * time.Second):
		t.Fatal("HandleQueue never reached <-queueWakeup within 2s")
	}

	return func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("HandleQueue did not exit in time")
		}
	}
}

// awaitEnqueued polls until ctx is found in queueEntries. Replaces
// time.Sleep-based synchronization.
func awaitEnqueued(t *testing.T, ctx context.Context) {
	t.Helper()
	testutil.WaitFor(t, 2*time.Second, "context enqueued", func() bool {
		queueLock.Lock()
		defer queueLock.Unlock()
		for _, e := range queueEntries {
			if e.ctx == ctx {
				return true
			}
		}
		return false
	})
}

func TestHandleQueueFIFO(t *testing.T) {
	resetQueueForTest(t)
	stop := startHandler(t)
	defer stop()

	// We enqueue one entry, let the handler fully process it (notify +
	// release), then enqueue the next. This avoids the wakeup-channel
	// race when the handler is parked inside a notify callback: a new
	// entry's wakeup would be silently dropped because the handler is
	// not yet back in the outer select.
	var (
		orderMu sync.Mutex
		order   []string
	)
	var wg sync.WaitGroup

	runEntry := func(label string) {
		entered := make(chan struct{})
		release := make(chan struct{})
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		wg.Add(1)
		go func() {
			defer wg.Done()
			WaitQueue(ctx, func() {
				orderMu.Lock()
				order = append(order, label)
				orderMu.Unlock()
				close(entered)
				<-release
			})
		}()

		select {
		case <-entered:
		case <-time.After(3 * time.Second):
			t.Fatalf("%s notify never fired", label)
		}
		close(release)
		cancel()

		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Fatalf("%s WaitQueue did not return", label)
		}
	}

	runEntry("1")
	runEntry("2")

	orderMu.Lock()
	defer orderMu.Unlock()
	if len(order) != 2 {
		t.Fatalf("expected 2 notifies, got %d: %v", len(order), order)
	}
	if order[0] != "1" || order[1] != "2" {
		t.Fatalf("expected FIFO order [1 2], got %v", order)
	}
}

func TestHandleQueueSkipsAlreadyDoneHead(t *testing.T) {
	resetQueueForTest(t)
	stop := startHandler(t)
	defer stop()

	var calls atomic.Int32

	headCtx, headCancel := context.WithCancel(context.Background())
	var cancelOnce sync.Once
	entry := &queueEntry{
		ctx:    headCtx,
		cancel: func() { cancelOnce.Do(func() {}) },
		notify: func() { calls.Add(1) },
	}
	queueLock.Lock()
	queueEntries = append(queueEntries, entry)
	queueLock.Unlock()

	headCancel()

	select {
	case queueWakeup <- struct{}{}:
	default:
	}

	testutil.WaitFor(t, 2*time.Second, "head removed", func() bool {
		queueLock.Lock()
		remaining := len(queueEntries)
		queueLock.Unlock()
		return remaining == 0
	})

	if got := calls.Load(); got != 0 {
		t.Errorf("notify called %d times for already-done head; want 0", got)
	}
}

func TestWaitQueueReturnsWhenCallerCtxCancelled(t *testing.T) {
	resetQueueForTest(t)
	// No handler running: we test WaitQueue in isolation.

	callerCtx, callerCancel := context.WithCancel(context.Background())
	returned := make(chan struct{})
	go func() {
		WaitQueue(callerCtx, nil)
		close(returned)
	}()
	awaitEnqueued(t, callerCtx)

	callerCancel()

	select {
	case <-returned:
	case <-time.After(2 * time.Second):
		t.Fatal("WaitQueue did not return after caller cancel")
	}
}

func TestHandleQueueExitsOnCtxCancel(t *testing.T) {
	resetQueueForTest(t)
	stop := startHandler(t)

	// No entries enqueued: the handler is parked in its outer
	// <-queueWakeup select. Cancelling its ctx must let it exit
	// within a short window.
	stop()
}

// TestHandleQueueGracefulShutdownFromOuter verifies that cancelling
// handler ctx while it is parked in the outer <-queueWakeup select
// triggers shutdownQueue: every WaitQueue caller parked in the queue
// returns within a short window even if their parent ctx is still alive.
//
// Note: there is intentionally no test for the in-flight-callback case
// (handler parked inside head.notify() when ctx is cancelled). The notify
// callback is invoked synchronously, so the handler cannot respond to
// ctx.Done() while it is on the callback's call stack -- a fundamental
// limitation of Go's synchronous call model. In production the callbacks
// are short (e.g. sending an SSE event), so the handler is effectively
// never blocked there for long. A future improvement would be to invoke
// notify in a goroutine and bound its wait during shutdown.
func TestHandleQueueGracefulShutdownFromOuter(t *testing.T) {
	resetQueueForTest(t)
	stop := startHandler(t)
	defer stop()

	// All callers park with non-blocking notifies (cb returns immediately)
	// so the handler can move through them between ctx.Done checks.
	var wg sync.WaitGroup
	const parked = 3
	for i := 0; i < parked; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			WaitQueue(context.Background(), nil)
		}()
	}

	// Wait for at least one caller to be parked, so the handler is
	// definitely inside its inner loop or between entries when we cancel.
	testutil.WaitFor(t, 2*time.Second, "no callers parked", func() bool {
		queueLock.Lock()
		n := len(queueEntries)
		queueLock.Unlock()
		return n >= 1
	})

	// Cancel the handler. With non-blocking notifies, the handler is
	// either in the outer select or between entries where the new
	// ctx.Done checks fire.
	stop()

	wgDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(wgDone)
	}()
	select {
	case <-wgDone:
	case <-time.After(2 * time.Second):
		t.Fatal("WaitQueue callers did not unblock after handler cancel")
	}
}

func TestGetQueuePosition(t *testing.T) {
	resetQueueForTest(t)

	// Empty queue: position is 0 (nothing found) and total reflects the
	// real queue size, which is 0 here. The previous bug returned (0, 0)
	// for non-empty queues too, hiding the queue's actual size from the
	// SSE consumer.
	if pos, total := GetQueuePositionByCtx(context.Background()); pos != 0 || total != 0 {
		t.Errorf("empty queue: got (%d, %d); want (0, 0)", pos, total)
	}

	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()

	queueLock.Lock()
	queueEntries = append(queueEntries,
		&queueEntry{ctx: ctx1},
		&queueEntry{ctx: ctx2},
	)
	queueLock.Unlock()

	tests := []struct {
		name      string
		ctx       context.Context
		wantPos   int
		wantTotal int
	}{
		{"first entry", ctx1, 1, 2},
		{"second entry", ctx2, 2, 2},
		// Regression: previously returned (0, 0), hiding the queue size
		// from the SSE consumer. Must report the real total so the UI
		// can show the true queue depth.
		{"unknown ctx reports real total", context.Background(), 0, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pos, total := GetQueuePositionByCtx(tt.ctx)
			if pos != tt.wantPos || total != tt.wantTotal {
				t.Errorf("got (%d, %d); want (%d, %d)", pos, total, tt.wantPos, tt.wantTotal)
			}
		})
	}
}

// TestGetQueuePositionCancelledContext pins the behaviour for a
// context that the caller has already cancelled but the queue has
// not yet pruned. The queue still holds the entry, so the function
// must report the entry's 1-based position and the real total.
func TestGetQueuePositionCancelledContext(t *testing.T) {
	resetQueueForTest(t)

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()

	queueLock.Lock()
	queueEntries = append(queueEntries, &queueEntry{ctx: cancelled})
	queueLock.Unlock()

	pos, total := GetQueuePositionByCtx(cancelled)
	if pos != 1 || total != 1 {
		t.Errorf("cancelled ctx in queue: got (%d, %d); want (1, 1)", pos, total)
	}
}

// TestGetQueuePositionFirstEntryAlwaysAtOne guards against a
// regression where FIFO ordering would be lost. The first enqueued
// entry must always report position 1, never 0.
func TestGetQueuePositionFirstEntryAlwaysAtOne(t *testing.T) {
	resetQueueForTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	queueLock.Lock()
	queueEntries = append(queueEntries, &queueEntry{ctx: ctx})
	queueLock.Unlock()

	pos, total := GetQueuePositionByCtx(ctx)
	if pos != 1 {
		t.Errorf("first entry pos = %d; want 1", pos)
	}
	if total != 1 {
		t.Errorf("first entry total = %d; want 1", total)
	}
}

// TestShutdownQueueReleasesAllWaiters verifies that the exported
// ShutdownQueue releases every parked WaitQueue caller immediately,
// regardless of their parent ctx. This is the graceful-shutdown
// safety net used by als.Init when SIGINT/SIGTERM arrives.
func TestShutdownQueueReleasesAllWaiters(t *testing.T) {
	resetQueueForTest(t)

	const parked = 3
	var wg sync.WaitGroup
	for i := 0; i < parked; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Parent ctx stays alive: only ShutdownQueue must unblock
			// these.
			WaitQueue(context.Background(), nil)
		}()
	}

	// Wait for all entries to be parked.
	testutil.WaitFor(t, 2*time.Second, "all callers parked", func() bool {
		queueLock.Lock()
		n := len(queueEntries)
		queueLock.Unlock()
		return n >= parked
	})

	ShutdownQueue()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("WaitQueue callers did not unblock after ShutdownQueue")
	}
}

func TestResetQueueForTest(t *testing.T) {
	// Pre-populate the queue with a parked entry.
	enqueueCtx, enqueueCancel := context.WithCancel(context.Background())
	entry := &queueEntry{ctx: enqueueCtx, cancel: enqueueCancel}
	queueLock.Lock()
	queueEntries = append(queueEntries, entry)
	queueLock.Unlock()

	ResetQueueForTest()

	queueLock.Lock()
	n := len(queueEntries)
	queueLock.Unlock()
	if n != 0 {
		t.Errorf("after ResetQueueForTest: len(queueEntries) = %d; want 0", n)
	}

	// The parked entry's queueCtx must be cancelled by the reset
	// (via ShutdownQueue).
	select {
	case <-enqueueCtx.Done():
	case <-time.After(time.Second):
		t.Error("parked entry's queueCtx was not cancelled by ResetQueueForTest")
	}
}

func TestResetQueueForTestOnEmpty(t *testing.T) {
	// Calling reset on an empty queue must not panic and must
	// leave the queue empty.
	queueLock.Lock()
	queueEntries = nil
	queueLock.Unlock()
	for {
		select {
		case <-queueWakeup:
		default:
			goto drained
		}
	}
drained:

	ResetQueueForTest()

	queueLock.Lock()
	n := len(queueEntries)
	queueLock.Unlock()
	if n != 0 {
		t.Errorf("after ResetQueueForTest on empty: len = %d; want 0", n)
	}
}

func TestWaitForHandlerParkedTimeoutWithoutHandler(t *testing.T) {
	// No handler is running. queueWakeup is unbuffered and the
	// goroutine inside WaitForHandlerParked blocks forever on
	// the first send. The outer select must time out and return
	// false within the deadline.
	//
	// Skip if the channel is somehow buffered (defensive: the
	// production declaration is unbuffered).
	if cap(queueWakeup) != 0 {
		t.Skip("queueWakeup is unexpectedly buffered; cannot exercise the timeout path")
	}

	start := time.Now()
	if WaitForHandlerParked(100 * time.Millisecond) {
		t.Fatal("WaitForHandlerParked returned true with no handler; want timeout")
	}
	elapsed := time.Since(start)
	if elapsed < 50*time.Millisecond {
		t.Errorf("returned too fast: %v; expected ~100ms", elapsed)
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("returned too slow: %v; expected ~100ms", elapsed)
	}
}
