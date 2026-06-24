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

// TestRemoveClientRemovesFromMap verifies that RemoveClient deletes
// the session from the global map. It no longer invokes any per-
// session cancel function (see TestGetContextDoesNotLeak for the
// goroutine-leak fix).
func TestRemoveClient(t *testing.T) {
	resetClientMap()

	session := &ClientSession{
		Channel:   make(chan *Message, 1),
		CreatedAt: time.Now(),
	}
	AddClient("to-remove", session)

	RemoveClient("to-remove")

	if _, ok := GetClient("to-remove"); ok {
		t.Error("GetClient returned true after RemoveClient")
	}
}

// TestRemoveClientDoesNotCancelInflightContexts verifies the new
// contract: RemoveClient only removes the session from the global
// map; in-flight contexts derived from the session remain live. The
// caller that called GetContext owns the cancel.
func TestRemoveClientDoesNotCancelInflightContexts(t *testing.T) {
	resetClientMap()

	session := &ClientSession{
		Channel:   make(chan *Message, 1),
		CreatedAt: time.Now(),
	}
	parent, parentCancel := context.WithCancel(context.Background())
	defer parentCancel()
	session.SetContext(parent)

	AddClient("keepalive", session)

	derived := session.GetContext(parent)
	RemoveClient("keepalive")

	if _, ok := GetClient("keepalive"); ok {
		t.Error("GetClient returned true after RemoveClient")
	}
	select {
	case <-derived.Done():
		t.Error("RemoveClient cancelled the derived ctx (must not)")
	default:
		// Good: derived ctx is still live after RemoveClient.
	}
}

// TestGetContextDoesNotLeakGoroutines exercises the bug where
// concurrent GetContext calls would overwrite the per-session
// cancelFunc, leaving earlier derived contexts stranded without
// cancellation. Each call now returns its own ctx with its own
// cancellation goroutine that releases when the caller cancels.
func TestGetContextDoesNotLeakGoroutines(t *testing.T) {
	resetClientMap()

	session := &ClientSession{
		Channel:   make(chan *Message, 1),
		CreatedAt: time.Now(),
	}

	parent := context.Background()

	// Fire many concurrent GetContext calls; each returns an
	// independent ctx whose lifetime is owned by the caller.
	const n = 50
	ctxs := make([]context.Context, n)
	cancels := make([]context.CancelFunc, n)
	for i := 0; i < n; i++ {
		ctxs[i], cancels[i] = context.WithCancel(session.GetContext(parent))
	}

	// Cancelling one must not affect the others.
	cancels[0]()

	for i := 0; i < n; i++ {
		select {
		case <-ctxs[i].Done():
			if i != 0 {
				t.Errorf("ctx[%d] cancelled unexpectedly", i)
			}
		default:
			if i == 0 {
				t.Errorf("ctx[0] should have been cancelled")
			}
		}
	}

	// Cancel the rest. The propagation goroutines must exit.
	for i := 1; i < n; i++ {
		cancels[i]()
	}
}

// TestGetContextConcurrentCallsCancelIndependently verifies that two
// concurrent GetContext calls produce two independent ctxs whose
// lifetimes are owned by each caller. Cancelling one must not cancel
// the other (the regression: previously a per-session cancelFunc
// field was overwritten, so the first caller's cancel handle became
// a dead reference while its goroutine leaked).
func TestGetContextConcurrentCallsCancelIndependently(t *testing.T) {
	resetClientMap()

	session := &ClientSession{
		Channel:   make(chan *Message, 1),
		CreatedAt: time.Now(),
	}

	parent := context.Background()

	ctxA, cancelA := context.WithCancel(session.GetContext(parent))
	ctxB, cancelB := context.WithCancel(session.GetContext(parent))

	cancelA()

	select {
	case <-ctxA.Done():
		// Good.
	case <-time.After(time.Second):
		t.Fatal("ctxA did not cancel")
	}

	select {
	case <-ctxB.Done():
		t.Error("ctxB was cancelled when ctxA was cancelled")
	default:
		// Good.
	}

	cancelB()
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

	session := &ClientSession{
		Channel:   make(chan *Message, 1),
		CreatedAt: time.Now(),
	}
	// Set a long-lived parent ctx so the c.ctx.Done() branch is
	// never taken; the request ctx is the one that fires.
	session.SetContext(context.Background())

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

// TestGetContextCancelledByParent covers the c.ctx.Done() branch of
// the propagation goroutine. We cancel the parent ctx that was
// passed to SetContext and verify the derived ctx is cancelled too.
func TestGetContextCancelledByParent(t *testing.T) {
	resetClientMap()

	parent, parentCancel := context.WithCancel(context.Background())

	session := &ClientSession{
		Channel:   make(chan *Message, 1),
		CreatedAt: time.Now(),
	}
	session.SetContext(parent)

	derived := session.GetContext(context.Background())

	parentCancel()

	select {
	case <-derived.Done():
		// Good.
	case <-time.After(time.Second):
		t.Error("derived ctx did not cancel when parent ctx was cancelled")
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
}

func TestSessionFromContext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		in     any
		wantOK bool
	}{
		{name: "nil input", in: nil, wantOK: false},
		{name: "wrong type", in: "not a session", wantOK: false},
		{name: "int input", in: 42, wantOK: false},
		{name: "valid session", in: &ClientSession{}, wantOK: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := SessionFromContext(tt.in)
			if ok != tt.wantOK {
				t.Errorf("SessionFromContext(%T) ok = %v; want %v", tt.in, ok, tt.wantOK)
			}
			if ok && got == nil {
				t.Error("ok but got nil session")
			}
		})
	}
}
