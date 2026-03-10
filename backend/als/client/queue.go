package client

import (
	"context"
	"sync"
)

type queueEntry struct {
	ctx    context.Context    // caller's parent context
	cancel context.CancelFunc // cancels the internal queueCtx
	notify func()            // optional position-update callback
}

var (
	queueLock    sync.Mutex
	queueEntries []*queueEntry        // ordered slice for FIFO
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

	// Wake up HandleQueue so it can process the new entry
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

func GetQueuePostitionByCtx(ctx context.Context) (int, int) {
	queueLock.Lock()
	defer queueLock.Unlock()

	total := len(queueEntries)
	for i, e := range queueEntries {
		if e.ctx == ctx {
			return i + 1, total
		}
	}
	return 0, 0
}

func HandleQueue() {
	for {
		<-queueWakeup

		for {
			// Take the head of queue (FIFO)
			queueLock.Lock()
			if len(queueEntries) == 0 {
				queueLock.Unlock()
				break
			}
			head := queueEntries[0]
			queueLock.Unlock()

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
			// so only one task runs at a time
			<-head.ctx.Done()

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
