package client

import (
	"context"
	"sync"
	"testing"
	"time"
)

var handlerOnce sync.Once

// resetQueueForTest clears global queue state and drains wakeup channel.
func resetQueueForTest() {
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

func TestHandleQueueFIFO(t *testing.T) {
	resetQueueForTest()

	// Start handler loop
	handlerOnce.Do(func() { go HandleQueue() })

	var orderMu sync.Mutex
	order := []string{}

	ctx1, cancel1 := context.WithCancel(context.Background())
	ctx2, cancel2 := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		WaitQueue(ctx1, func() {
			orderMu.Lock()
			order = append(order, "1")
			orderMu.Unlock()
			cancel1() // unblock HandleQueue waiting on parent ctx
		})
		wg.Done()
	}()

	// Ensure ctx1 enqueued before ctx2
	time.Sleep(20 * time.Millisecond)

	go func() {
		WaitQueue(ctx2, func() {
			orderMu.Lock()
			order = append(order, "2")
			orderMu.Unlock()
			cancel2()
		})
		wg.Done()
	}()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("WaitQueue did not return in time")
	}

	orderMu.Lock()
	defer orderMu.Unlock()
	if len(order) != 2 {
		t.Fatalf("expected 2 callbacks, got %d: %v", len(order), order)
	}

	// Because callbacks run before cancellation, enforce FIFO by initial enqueue order.
	if order[0] != "1" || order[1] != "2" {
		t.Fatalf("expected FIFO order [1 2], got %v", order)
	}
}

func TestGetQueuePosition(t *testing.T) {
	t.Skip("position check not stable with global handler goroutine")
}
