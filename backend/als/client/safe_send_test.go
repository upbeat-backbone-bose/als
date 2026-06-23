package client

import (
	"context"
	"testing"
	"time"
)

func TestSafeChannelSend(t *testing.T) {
	tests := []struct {
		name        string
		capacity    int
		fillChannel bool
		ctxDone     bool
		nilCh       bool
		nilCtx      bool
		wantSent    bool
	}{
		{
			name:     "succeeds when channel has room",
			capacity: 1,
			wantSent: true,
		},
		{
			name:        "drops when channel is full",
			capacity:    1,
			fillChannel: true,
			wantSent:    false,
		},
		{
			name:     "drops when ctx cancelled and channel is full",
			capacity: 1,
			fillChannel: true,
			ctxDone:  true,
			wantSent: false,
		},
		{
			name:     "nil channel returns false",
			capacity: 0,
			nilCh:    true,
			wantSent: false,
		},
		{
			name:     "nil ctx falls back to background",
			capacity: 1,
			nilCtx:   true,
			wantSent: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var ch chan *Message
			if !tt.nilCh {
				ch = make(chan *Message, tt.capacity)
			}

			var ctx context.Context
			var cancel context.CancelFunc
			if !tt.nilCtx {
				ctx, cancel = context.WithCancel(context.Background())
				defer cancel()
			}
			if tt.ctxDone && cancel != nil {
				cancel()
			}

			if tt.fillChannel && !tt.nilCh && tt.capacity > 0 {
				ch <- &Message{Name: "filler"}
			}

			got := SafeChannelSend(ctx, ch, &Message{Name: "x"})
			if got != tt.wantSent {
				t.Errorf("SafeChannelSend() = %v; want %v", got, tt.wantSent)
			}
		})
	}
}

func TestSafeChannelSendNeverBlocks(t *testing.T) {
	// Fill a buffered channel completely; further SafeChannelSend calls
	// must return immediately rather than block waiting for room.
	t.Parallel()

	ch := make(chan *Message, 1)
	ch <- &Message{Name: "filler"} // channel full

	done := make(chan bool, 1)
	go func() {
		SafeChannelSend(context.Background(), ch, &Message{Name: "x"})
		done <- true
	}()

	select {
	case <-done:
		// Good: returned promptly.
	case <-time.After(time.Second):
		t.Fatal("SafeChannelSend blocked on a full channel")
	}
}

// TestSafeChannelSendClientSession exercises the helper through the
// actual ClientSession API to catch any drift between Message /
// Channel types and the helper signature.
func TestSafeChannelSendClientSession(t *testing.T) {
	t.Parallel()

	session := &ClientSession{
		Channel:   make(chan *Message, 1),
		CreatedAt: time.Now(),
	}
	if !SafeChannelSend(context.Background(), session.Channel, &Message{Name: "hello"}) {
		t.Fatal("first send should succeed")
	}
	if SafeChannelSend(context.Background(), session.Channel, &Message{Name: "dropped"}) {
		t.Fatal("second send should drop because channel is full")
	}
}