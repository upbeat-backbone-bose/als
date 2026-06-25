package client

import (
	"context"
	"sync"
	"time"
)

type queueEntry struct {
	ctx    context.Context    // caller's parent context
	cancel context.CancelFunc // cancels the internal queueCtx
	notify func()             // optional position-update callback
}

var (
	queueLock    sync.Mutex
	queueEntries []*queueEntry         // ordered slice for FIFO
	queueWakeup  = make(chan struct{}) // signal to HandleQueue
)

func WaitQueue(ctx context.Context, cb func()) {
	queueCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	entry := &queueEntry{
		ctx:    ctx,
		cancel: cancel,
		notify: cb,
	}

	queueLock.Lock()
	queueEntries = append(queueEntries, entry)
	queueLock.Unlock()

	defer func() {
		queueLock.Lock()
		for i, e := range queueEntries {
			if e == entry {
				queueEntries = append(queueEntries[:i], queueEntries[i+1:]...)
				break
			}
		}
		queueLock.Unlock()
	}()

	// Wake up HandleQueue so it can process the new entry.
	//
	// The send is non-blocking on purpose: if HandleQueue is parked inside
	// a long-running notify callback, the new caller must not block here.
	// HandleQueue's inner loop self-drains queueEntries until empty, so
	// even a lost wakeup is fine -- the new entry will be picked up as
	// soon as the previous callback returns.
	select {
	case queueWakeup <- struct{}{}:
	default:
	}

	// Block until queueCtx is cancelled (by HandleQueue) or parent ctx is done
	select {
	case <-queueCtx.Done():
	case <-ctx.Done():
	}
}

// ShutdownQueue cancels every entry's queueCtx so parked WaitQueue calls
// unblock immediately. Used during graceful shutdown to release callers
// even if their parent ctx is still alive.
func ShutdownQueue() {
	queueLock.Lock()
	defer queueLock.Unlock()
	for _, e := range queueEntries {
		if e != nil && e.cancel != nil {
			e.cancel()
		}
	}
}

// WaitForHandlerParked is a test-only helper that signals
// queueWakeup twice and waits for both sends to complete, which
// only succeeds once the HandleQueue goroutine is parked on its
// outer <-queueWakeup select. Callers should run their HandleQueue
// goroutine before calling this. Returns true on success, false
// on timeout.
func WaitForHandlerParked(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	done := make(chan struct{})
	go func() {
		defer close(done)
		queueWakeup <- struct{}{}
		queueWakeup <- struct{}{}
	}()
	select {
	case <-done:
		return true
	case <-time.After(time.Until(deadline)):
		return false
	}
}

// ResetQueueForTest is a test-only helper that cancels any pending
// entries and clears the queue slice plus drains the wakeup
// channel. It is safe to call from any package's _test.go.
func ResetQueueForTest() {
	ShutdownQueue()
	queueLock.Lock()
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

func GetQueuePositionByCtx(ctx context.Context) (position, total int) {
	queueLock.Lock()
	defer queueLock.Unlock()

	total = len(queueEntries)
	for i, e := range queueEntries {
		if e.ctx == ctx {
			return i + 1, total
		}
	}
	return 0, total
}

// HandleQueue drains the global FIFO queue until ctx is cancelled.
// Cancellation makes the outer loop exit promptly; in-flight entries
// already being processed are allowed to finish.
func HandleQueue(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			ShutdownQueue()
			return
		case <-queueWakeup:
		}

		for {
			// Take the head of queue (FIFO)
			queueLock.Lock()
			if len(queueEntries) == 0 {
				queueLock.Unlock()
				break
			}
			head := queueEntries[0]
			queueLock.Unlock()

			// Bail out promptly if cancellation arrives between entries.
			select {
			case <-ctx.Done():
				ShutdownQueue()
				return
			default:
			}

			// Check if the head's parent context is already done
			select {
			case <-head.ctx.Done():
				// Caller already gone; remove and skip
				queueLock.Lock()
				if len(queueEntries) > 0 && queueEntries[0] == head {
					queueEntries = queueEntries[1:]
				}
				queueLock.Unlock()
				continue
			default:
			}

			// Notify the head entry's callback (if any)
			if head.notify != nil {
				head.notify()
			}

			// Release the head: cancel its internal queueCtx so WaitQueue returns
			head.cancel()

			// Wait for the caller's task to finish (parent ctx done),
			// so only one task runs at a time. Also bail out promptly if
			// our own ctx is cancelled so graceful shutdown is not blocked
			// by a caller that hangs inside the notify callback.
			select {
			case <-head.ctx.Done():
			case <-ctx.Done():
				ShutdownQueue()
				return
			}

			// Clean up the head
			queueLock.Lock()
			if len(queueEntries) > 0 && queueEntries[0] == head {
				queueEntries = queueEntries[1:]
			}

			// Notify remaining entries of their updated queue position
			for _, e := range queueEntries {
				if e.notify != nil {
					e.notify()
				}
			}
			queueLock.Unlock()
		}
	}
}
