package client

import (
	"context"
	"sync"
)

var (
	clientsMu sync.RWMutex
	Clients   = make(map[string]*ClientSession)
)

type Message struct {
	Name    string
	Content string
}

type ClientSession struct {
	Channel chan *Message
	ctx     context.Context
}

func (c *ClientSession) SetContext(ctx context.Context) {
	c.ctx = ctx
}

func (c *ClientSession) GetContext(requestCtx context.Context) context.Context {
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		select {
		case <-c.ctx.Done():
			cancel()
			break
		case <-requestCtx.Done():
			cancel()
			break
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
	return session, ok
}

func RemoveClient(id string) {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	delete(Clients, id)
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
