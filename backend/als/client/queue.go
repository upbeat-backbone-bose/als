package client

import (
	"context"
	"sync"
)

type queueEntry struct {
	ctx    context.Context    // caller's parent context
	cancel context.CancelFunc // cancels the internal queueCtx
	notify func()             // optional position-update callback
}

var (
	queueLock         sync.Mutex
	queueEntries      []*queueEntry            // ordered slice for FIFO
	queueWakeup       = make(chan struct{}, 1) // signal to HandleQueue; capacity 1 prevents lost wakeups
	queueShutdown     = make(chan struct{})    // closed once on shutdown
	queueShutdownOnce sync.Once
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

	// Block until queueCtx is cancelled (by HandleQueue), parent ctx is
	// done, or the queue itself is shut down.
	select {
	case <-queueCtx.Done():
	case <-ctx.Done():
	case <-queueShutdown:
	}
}

// ShutdownQueue cancels every entry's queueCtx so parked WaitQueue calls
// unblock immediately.  It also closes queueShutdown so that callers who
// enqueue after the shutdown begins — a window that exists because
// ShutdownQueue releases queueLock before the caller's goroutine truly
// returns — unblock through the closed channel rather than waiting
// forever on a queueCtx that will never be cancelled.
func ShutdownQueue() {
	queueShutdownOnce.Do(func() { close(queueShutdown) })

	queueLock.Lock()
	defer queueLock.Unlock()
	for _, e := range queueEntries {
		if e != nil && e.cancel != nil {
			e.cancel()
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
	// Safety net: when HandleQueue returns, cancel every entry still
	// parked in the queue.  The explicit ShutdownQueue calls below
	// release entries that are visible at the moment ctx is cancelled;
	// this deferred call catches stragglers that enqueue between the
	// explicit call and the function epilogue — a window that exists
	// because ShutdownQueue releases the lock before HandleQueue's
	// goroutine truly exits.
	defer ShutdownQueue()
	for {
		// Double-check: entries may have arrived between the inner
		// loop's last Unlock and the outer select. WaitQueue sends
		// wakeup via a non-blocking channel write that drops when
		// HandleQueue is not parked. Re-checking under lock catches
		// these stragglers without needing an extra wakeup signal.
		queueLock.Lock()
		if len(queueEntries) > 0 {
			queueLock.Unlock()
			goto drain
		}
		queueLock.Unlock()

		select {
		case <-ctx.Done():
			ShutdownQueue()
			return
		case <-queueWakeup:
		}

	drain:
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
