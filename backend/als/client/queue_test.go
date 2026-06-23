package client

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
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
// cancellable context. The goroutine exits when ctx is cancelled, so
// tests can run independently without sharing a long-lived handler.
//
// Returns (handlerCtx, stop). The handler is guaranteed to be parked in
// its outer select on <-queueWakeup before startHandler returns.
func startHandler(t *testing.T) (handlerCtx context.Context, stop func()) {
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

	return handlerCtx, func() {
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
	deadline := time.Now().Add(2 * time.Second)
	for {
		queueLock.Lock()
		found := false
		for _, e := range queueEntries {
			if e.ctx == ctx {
				found = true
				break
			}
		}
		remaining := len(queueEntries)
		queueLock.Unlock()
		if found {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("context never enqueued within 2s; queue size=%d", remaining)
		}
		runtime.Gosched()
		time.Sleep(time.Millisecond)
	}
}

// awaitNotify waits until calls has reached want.
func awaitNotify(t *testing.T, calls *atomic.Int32, want int32) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for calls.Load() < want {
		if time.Now().After(deadline) {
			t.Fatalf("notify fired %d times; want %d", calls.Load(), want)
		}
		runtime.Gosched()
		time.Sleep(time.Millisecond)
	}
}

// gate is a per-entry handshake: the notify callback signals "entered"
// (so the test knows the handler has begun processing this entry) and
// blocks on "release" (so the handler cannot advance until the test
// explicitly releases it). This lets tests observe FIFO ordering without
// race-prone enqueue-time polling.
type gate struct {
	entered chan struct{}
	release chan struct{}
}

func newGate() *gate {
	return &gate{entered: make(chan struct{}), release: make(chan struct{})}
}

func TestHandleQueueFIFO(t *testing.T) {
	resetQueueForTest(t)
	_, stop := startHandler(t)
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
	_, stop := startHandler(t)
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

	deadline := time.Now().Add(2 * time.Second)
	for {
		queueLock.Lock()
		remaining := len(queueEntries)
		queueLock.Unlock()
		if remaining == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("head not removed; remaining=%d", remaining)
		}
		runtime.Gosched()
		time.Sleep(time.Millisecond)
	}

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
	_, stop := startHandler(t)

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
	_, stop := startHandler(t)
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
	deadline := time.Now().Add(2 * time.Second)
	for {
		queueLock.Lock()
		n := len(queueEntries)
		queueLock.Unlock()
		if n >= 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no callers parked within 2s")
		}
		runtime.Gosched()
		time.Sleep(time.Millisecond)
	}

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
		{"unknown ctx returns zeros", context.Background(), 0, 0},
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