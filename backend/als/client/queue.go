package client

import (
	"context"
	"sync"
)

type queueEntry struct {
	ctx    context.Context
	cancel context.CancelFunc
	notify func()
}

type QueueManager struct {
	mu         sync.Mutex
	entries    []*queueEntry
	wakeup     chan struct{}
	ctx        context.Context
	cancelFunc context.CancelFunc
}

func NewQueueManager() *QueueManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &QueueManager{
		entries:    make([]*queueEntry, 0),
		wakeup:     make(chan struct{}, 1),
		ctx:        ctx,
		cancelFunc: cancel,
	}
}

func (m *QueueManager) Wakeup() {
	select {
	case m.wakeup <- struct{}{}:
	default:
	}
}

func (m *QueueManager) WaitQueue(ctx context.Context, cb func()) {
	queueCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	entry := &queueEntry{
		ctx:    ctx,
		cancel: cancel,
		notify: cb,
	}

	m.mu.Lock()
	m.entries = append(m.entries, entry)
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		for i, e := range m.entries {
			if e == entry {
				m.entries = append(m.entries[:i], m.entries[i+1:]...)
				break
			}
		}
		m.mu.Unlock()
	}()

	m.Wakeup()

	select {
	case <-queueCtx.Done():
	case <-ctx.Done():
	}
}

func (m *QueueManager) GetQueuePositionByCtx(ctx context.Context) (int, int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	total := len(m.entries)
	for i, e := range m.entries {
		if e.ctx == ctx {
			return i + 1, total
		}
	}
	return 0, 0
}

func (m *QueueManager) HandleQueue() {
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-m.wakeup:
		}

		for {
			m.mu.Lock()
			if len(m.entries) == 0 {
				m.mu.Unlock()
				break
			}
			head := m.entries[0]
			m.mu.Unlock()

			select {
			case <-head.ctx.Done():
				m.mu.Lock()
				if len(m.entries) > 0 && m.entries[0] == head {
					m.entries = m.entries[1:]
				}
				m.mu.Unlock()
				continue
			default:
			}

			if head.notify != nil {
				head.notify()
			}

			head.cancel()

			<-head.ctx.Done()

			m.mu.Lock()
			if len(m.entries) > 0 && m.entries[0] == head {
				m.entries = m.entries[1:]
			}

			for _, e := range m.entries {
				if e.notify != nil {
					e.notify()
				}
			}
			m.mu.Unlock()
		}
	}
}

func (m *QueueManager) Shutdown() {
	m.cancelFunc()
}
