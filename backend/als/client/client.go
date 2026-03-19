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

type Message struct {
	Name    string
	Content string
}

type ClientSession struct {
	Channel    chan *Message
	ctx        context.Context
	CreatedAt  time.Time
	cancelFunc context.CancelFunc
}

func (c *ClientSession) SetContext(ctx context.Context) {
	c.ctx = ctx
}

func (c *ClientSession) GetContext(requestCtx context.Context) context.Context {
	ctx, cancel := context.WithCancel(requestCtx)
	c.cancelFunc = cancel
	go func() {
		select {
		case <-c.ctx.Done():
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

func RemoveClient(id string) {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	if client, ok := Clients[id]; ok && client.cancelFunc != nil {
		client.cancelFunc()
	}
	delete(Clients, id)
}

func RemoveExpiredClients() int {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	expired := time.Now().Add(-sessionExpireDuration)
	removed := 0
	for id, session := range Clients {
		if session.CreatedAt.Before(expired) {
			if session.cancelFunc != nil {
				session.cancelFunc()
			}
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

func BroadCastMessage(name string, content string) {
	msg := &Message{
		Name:    name,
		Content: content,
	}
	for _, client := range SnapshotClients() {
		client.TrySend(msg)
	}
}
