package client

import (
	"context"
	"sync"
	"time"
)

const sessionExpireDuration = 24 * time.Hour

var (
	clientsMu sync.RWMutex
	Clients   = make(map[string]*ClientSession)
)

// ClientsMu exposes the package-internal mutex to tests in other
// packages that need to seed/inspect Clients without racing. Production
// code should use the higher-level helpers (AddClient, GetClient,
// RemoveExpiredClients, etc.) instead.
func ClientsMu() *sync.RWMutex { return &clientsMu }

type Message struct {
	Name    string
	Content string
}

type ClientSession struct {
	Channel chan *Message
	ctx     context.Context
	// done is closed by Close to signal the session's consumers
	// (the SSE handler, the timer broadcast loop) that the
	// session has been taken over or torn down. It is a distinct
	// mechanism from ctx so the resume path can stop the old SSE
	// handler without touching the ctx chain that other request
	// handlers depend on. Lazily initialised by ensureDone so the
	// existing "struct literal + AddClient" call sites keep
	// working without changes.
	done      chan struct{}
	initOnce  sync.Once
	closeOnce sync.Once
	CreatedAt time.Time
}

func (c *ClientSession) SetContext(ctx context.Context) {
	c.ctx = ctx
}

// ensureDone lazily creates the done channel. Idempotent and safe
// under concurrent first-callers thanks to initOnce.
func (c *ClientSession) ensureDone() {
	c.initOnce.Do(func() {
		c.done = make(chan struct{})
	})
}

// Done returns a channel that is closed by Close. Consumers (the
// SSE handler in particular) select on it to break out of their
// read loop when the session has been taken over by a resume or
// otherwise torn down. The returned channel is never closed twice
// and is always non-nil after the first call.
func (c *ClientSession) Done() <-chan struct{} {
	c.ensureDone()
	return c.done
}

// Close signals that the session should be torn down. Subsequent
// calls are no-ops. This is the resume path's lever: the new
// /session handler calls Close on the old ClientSession, the old
// SSE handler observes it via Done(), and exits its select loop.
// The old handler's defer still runs but uses IsCurrent to avoid
// removing the freshly-installed replacement entry from the map.
func (c *ClientSession) Close() {
	c.ensureDone()
	c.closeOnce.Do(func() { close(c.done) })
}

// GetContext returns a derived context that is cancelled when either
// the parent context (requestCtx) or the session's parent context
// (set via SetContext) is cancelled. Each call returns an independent
// context with its own cancellation goroutine -- callers MUST defer
// the returned cancel to release the goroutine promptly.
func (c *ClientSession) GetContext(requestCtx context.Context) context.Context {
	ctx, cancel := context.WithCancel(requestCtx)
	go func() {
		// Local references: c.ctx may be reassigned by SetContext
		// concurrently, so we capture the current parent here.
		parent := c.ctx
		var parentDone <-chan struct{}
		if parent != nil {
			parentDone = parent.Done()
		}
		select {
		case <-parentDone:
			cancel()
		case <-ctx.Done():
			cancel()
		}
	}()
	return ctx
}

func (c *ClientSession) TrySend(msg *Message) bool {
	select {
	case c.Channel <- msg:
		return true
	default:
		return false
	}
}

// SessionFromContext extracts the ClientSession previously stored under
// the "clientSession" key by the session middleware. It returns the
// session and true on success, or (nil, false) if the value is missing
// or has an unexpected type. Callers should treat the false case as a
// programming error (middleware not installed) and return 500.
func SessionFromContext(v any) (*ClientSession, bool) {
	s, ok := v.(*ClientSession)
	return s, ok
}

func AddClient(id string, session *ClientSession) {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	Clients[id] = session
}

// IsCurrent reports whether the entry stored under id is the same
// pointer as s. The resume path uses this to let a stale SSE
// handler's defer skip RemoveClient: after the resume replaces the
// map entry, the old handler's clientSession pointer is no longer
// the one the map points at, so IsCurrent returns false and the
// old handler leaves the new entry in place.
func IsCurrent(id string, s *ClientSession) bool {
	clientsMu.RLock()
	defer clientsMu.RUnlock()
	return Clients[id] == s
}

func GetClient(id string) (*ClientSession, bool) {
	clientsMu.RLock()
	defer clientsMu.RUnlock()
	session, ok := Clients[id]
	if ok && time.Since(session.CreatedAt) > sessionExpireDuration {
		return nil, false
	}
	return session, ok
}

// RemoveClient deletes the session from the global map. It does NOT
// cancel any in-flight contexts derived from this session -- the
// caller that called GetContext owns the returned context and is
// expected to defer its cancel. Forcing cancellation here would
// cause goroutine leaks in the legitimate concurrent-call pattern
// (ping and iperf3 both call GetContext multiple times per request).
func RemoveClient(id string) {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	delete(Clients, id)
}

// RemoveAllClients empties the global Clients map. Test-only: used
// to isolate tests that broadcast, since the package-level map is
// otherwise shared across the test binary. Holding the write lock
// is required to avoid racing with AddClient, RemoveClient, and
// any code that iterates the map under clientsMu.
func RemoveAllClients() {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	Clients = make(map[string]*ClientSession)
}

// RemoveExpiredClients deletes sessions older than sessionExpireDuration
// from the global map. As with RemoveClient, it does not cancel any
// in-flight contexts -- the original callers own those.
func RemoveExpiredClients() int {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	expired := time.Now().Add(-sessionExpireDuration)
	removed := 0
	for id, session := range Clients {
		if session.CreatedAt.Before(expired) {
			delete(Clients, id)
			removed++
		}
	}
	return removed
}

func SnapshotClients() []*ClientSession {
	clientsMu.RLock()
	defer clientsMu.RUnlock()
	list := make([]*ClientSession, 0, len(Clients))
	for _, c := range Clients {
		list = append(list, c)
	}
	return list
}

func BroadCastMessage(name, content string) {
	msg := &Message{
		Name:    name,
		Content: content,
	}
	for _, client := range SnapshotClients() {
		client.TrySend(msg)
	}
}
