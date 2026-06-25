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
	Channel   chan *Message
	ctx       context.Context
	CreatedAt time.Time
}

func (c *ClientSession) SetContext(ctx context.Context) {
	c.ctx = ctx
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
		// (See the package doc on c.ctx for the threading model.)
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
	if v == nil {
		return nil, false
	}
	s, ok := v.(*ClientSession)
	return s, ok
}

func AddClient(id string, session *ClientSession) {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	Clients[id] = session
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
