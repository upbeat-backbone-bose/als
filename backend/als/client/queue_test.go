package client

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestHandleQueueFIFO(t *testing.T) {
	queueMgr := NewQueueManager()
	go queueMgr.HandleQueue()
	defer queueMgr.Shutdown()

	var orderMu sync.Mutex
	order := []string{}

	ctx1, cancel1 := context.WithCancel(context.Background())
	ctx2, cancel2 := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		queueMgr.WaitQueue(ctx1, func() {
			orderMu.Lock()
			order = append(order, "1")
			orderMu.Unlock()
			cancel1()
		})
		wg.Done()
	}()

	time.Sleep(20 * time.Millisecond)

	go func() {
		queueMgr.WaitQueue(ctx2, func() {
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

	if order[0] != "1" || order[1] != "2" {
		t.Fatalf("expected FIFO order [1 2], got %v", order)
	}
}

func TestClientManager_AddAndGet(t *testing.T) {
	mgr := NewClientManager()
	
	session := &ClientSession{
		Channel:   make(chan *Message, 10),
		CreatedAt: time.Now(),
	}
	
	mgr.AddClient("test-id", session)
	
	got, ok := mgr.GetClient("test-id")
	if !ok {
		t.Fatal("Expected to find session")
	}
	if got != session {
		t.Fatal("Expected same session instance")
	}
}

func TestClientManager_Remove(t *testing.T) {
	mgr := NewClientManager()
	
	closed := false
	session := &ClientSession{
		Channel:   make(chan *Message, 10),
		CreatedAt: time.Now(),
		onClose:   func() { closed = true },
	}
	
	mgr.AddClient("test-id", session)
	mgr.RemoveClient("test-id")
	
	if !closed {
		t.Fatal("Expected onClose callback to be called")
	}
	
	_, ok := mgr.GetClient("test-id")
	if ok {
		t.Fatal("Expected session to be removed")
	}
}
