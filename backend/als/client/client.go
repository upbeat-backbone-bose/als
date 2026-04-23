package client

import (
	"context"
	"sync"
	"time"
)

const sessionExpireDuration = 24 * time.Hour

type Message struct {
	Name    string
	Content string
}

type ClientSession struct {
	Channel   chan *Message
	CreatedAt time.Time
	ctx       context.Context
	onClose   func()
}

func NewClientSession(ctx context.Context, onClose func()) *ClientSession {
	return &ClientSession{
		Channel:   make(chan *Message, 64),
		CreatedAt: time.Now(),
		ctx:       ctx,
		onClose:   onClose,
	}
}

func (c *ClientSession) Context() context.Context {
	return c.ctx
}

func (c *ClientSession) SetOnClose(fn func()) {
	c.onClose = fn
}

func (c *ClientSession) Close() {
	if c.onClose != nil {
		c.onClose()
	}
}

func (c *ClientSession) TrySend(msg *Message) bool {
	select {
	case c.Channel <- msg:
		return true
	default:
		return false
	}
}

type ClientManager struct {
	mu       sync.RWMutex
	clients  map[string]*ClientSession
	queueMgr *QueueManager
}

func NewClientManager() *ClientManager {
	return &ClientManager{
		clients:  make(map[string]*ClientSession),
		queueMgr: NewQueueManager(),
	}
}

func (m *ClientManager) AddClient(id string, session *ClientSession) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clients[id] = session
}

func (m *ClientManager) GetClient(id string) (*ClientSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	session, ok := m.clients[id]
	if ok && time.Since(session.CreatedAt) > sessionExpireDuration {
		return nil, false
	}
	return session, ok
}

func (m *ClientManager) RemoveClient(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if client, ok := m.clients[id]; ok {
		client.Close()
	}
	delete(m.clients, id)
}

func (m *ClientManager) RemoveExpiredClients() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	expired := time.Now().Add(-sessionExpireDuration)
	removed := 0
	for id, session := range m.clients {
		if session.CreatedAt.Before(expired) {
			session.Close()
			delete(m.clients, id)
			removed++
		}
	}
	return removed
}

func (m *ClientManager) SnapshotClients() []*ClientSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	list := make([]*ClientSession, 0, len(m.clients))
	for _, c := range m.clients {
		list = append(list, c)
	}
	return list
}

func (m *ClientManager) BroadCastMessage(name string, content string) {
	msg := &Message{
		Name:    name,
		Content: content,
	}
	for _, client := range m.SnapshotClients() {
		client.TrySend(msg)
	}
}

func (m *ClientManager) WaitQueue(ctx context.Context, cb func()) {
	m.queueMgr.WaitQueue(ctx, cb)
}

func (m *ClientManager) GetQueuePositionByCtx(ctx context.Context) (int, int) {
	return m.queueMgr.GetQueuePositionByCtx(ctx)
}

func (m *ClientManager) StartQueueHandler() {
	m.queueMgr.HandleQueue()
}

func (m *ClientManager) ShutdownQueue() {
	m.queueMgr.Shutdown()
}
