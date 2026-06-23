package client

import (
	"context"
	"testing"
	"time"
)

func TestAddAndGetClient(t *testing.T) {
	resetClientMap()

	session := &ClientSession{
		Channel:   make(chan *Message, 1),
		CreatedAt: time.Now(),
	}
	AddClient("alpha", session)

	got, ok := GetClient("alpha")
	if !ok {
		t.Fatal("GetClient(alpha) returned false")
	}
	if got != session {
		t.Errorf("got = %p; want %p", got, session)
	}

	if _, ok := GetClient("nope"); ok {
		t.Error("GetClient(nope) should return false")
	}
}

func TestRemoveClient(t *testing.T) {
	resetClientMap()

	cancelCalled := make(chan struct{}, 1)
	session := &ClientSession{
		Channel:   make(chan *Message, 1),
		CreatedAt: time.Now(),
		cancelFunc: func() {
			select {
			case cancelCalled <- struct{}{}:
			default:
			}
		},
	}
	AddClient("to-remove", session)

	RemoveClient("to-remove")

	if _, ok := GetClient("to-remove"); ok {
		t.Error("GetClient returned true after RemoveClient")
	}
	select {
	case <-cancelCalled:
	case <-time.After(time.Second):
		t.Error("RemoveClient did not invoke cancelFunc")
	}
}

func TestRemoveClientMissing(t *testing.T) {
	resetClientMap()
	// Removing a non-existent id must not panic.
	RemoveClient("never-existed")
}

func TestRemoveClientNoCancelFunc(t *testing.T) {
	resetClientMap()
	// cancelFunc nil -- the remove path must not panic.
	session := &ClientSession{
		Channel:   make(chan *Message, 1),
		CreatedAt: time.Now(),
	}
	AddClient("no-cancel", session)
	RemoveClient("no-cancel")
}

func TestSetContextAndGetContext(t *testing.T) {
	resetClientMap()

	parent, parentCancel := context.WithCancel(context.Background())
	defer parentCancel()

	session := &ClientSession{
		Channel:   make(chan *Message, 1),
		CreatedAt: time.Now(),
	}

	// SetContext stores a parent ctx.
	inner, innerCancel := context.WithCancel(context.Background())
	defer innerCancel()
	session.SetContext(inner)

	// GetContext returns a derived ctx. Cancelling inner must
	// propagate through GetContext's own goroutine into the
	// derived ctx.
	derived := session.GetContext(parent)

	select {
	case <-derived.Done():
		t.Fatal("derived ctx is already done")
	default:
	}

	innerCancel()

	select {
	case <-derived.Done():
		// Good: cancellation propagated.
	case <-time.After(time.Second):
		t.Error("GetContext did not cancel its derived ctx when SetContext parent was cancelled")
	}
}

func TestGetContextCancelledByRequest(t *testing.T) {
	resetClientMap()

	inner, innerCancel := context.WithCancel(context.Background())
	defer innerCancel()

	session := &ClientSession{
		Channel:   make(chan *Message, 1),
		CreatedAt: time.Now(),
	}
	session.SetContext(inner)

	req, reqCancel := context.WithCancel(context.Background())
	derived := session.GetContext(req)

	reqCancel()

	select {
	case <-derived.Done():
		// Good.
	case <-time.After(time.Second):
		t.Error("derived ctx did not cancel when request ctx was cancelled")
	}
}

// resetClientMap empties Clients without invoking cancelFuncs --
// use at the start of a test that sets up its own sessions.
func resetClientMap() {
	clientsMu.Lock()
	Clients = make(map[string]*ClientSession)
	clientsMu.Unlock()
}

func TestClientsMuExposesInternalMutex(t *testing.T) {
	mu := ClientsMu()
	if mu == nil {
		t.Fatal("ClientsMu returned nil")
	}
	mu.Lock()
	mu.Unlock()
}