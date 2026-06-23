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
	resetQueueForTest()

	// empty queue
	if pos, total := GetQueuePositionByCtx(context.Background()); pos != 0 || total != 0 {
		t.Errorf("empty queue: got (%d, %d); want (0, 0)", pos, total)
	}

	// Seed two entries with two distinct ctxs, look up by ctx pointer.
	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()

	// Enqueue directly (bypass WaitQueue to avoid blocking).
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
