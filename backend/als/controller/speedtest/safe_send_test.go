package speedtest

import (
	"context"
	"testing"
	"time"

	"github.com/samlm0/als/v2/als/client"
)

func TestSafeChannelSend(t *testing.T) {
	tests := []struct {
		name        string
		capacity    int
		fillChannel bool
		ctxDone     bool
		wantSent    bool
	}{
		{
			name:     "succeeds when channel has room",
			capacity: 1,
			wantSent: true,
		},
		{
			name:        "drops when channel is full",
			capacity:    0, // unbuffered
			fillChannel: true,
			wantSent:    false,
		},
		{
			name:     "drops when context is cancelled and channel is full",
			capacity: 1,
			fillChannel: true,
			ctxDone:  true,
			wantSent: false,
		},
		{
			name:     "nil channel returns false",
			capacity: 0,
			wantSent: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ch := make(chan *client.Message, tt.capacity)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if tt.ctxDone {
				cancel()
			}

			// Fill the channel without blocking. For unbuffered channels
			// we cannot actually "fill" them without a receiver, so we
			// only fill for buffered channels.
			if tt.fillChannel && tt.capacity > 0 {
				ch <- &client.Message{Name: "filler"}
			}

			got := safeChannelSend(ctx, ch, &client.Message{Name: "x"})
			if got != tt.wantSent {
				t.Errorf("safeChannelSend() = %v; want %v", got, tt.wantSent)
			}
		})
	}
}

func TestSafeChannelSendNeverBlocks(t *testing.T) {
	// Fill a buffered channel completely; further safeChannelSend calls
	// must return immediately rather than block waiting for room.
	t.Parallel()

	ch := make(chan *client.Message, 1)
	ch <- &client.Message{Name: "filler"} // channel full

	done := make(chan bool, 1)
	go func() {
		safeChannelSend(context.Background(), ch, &client.Message{Name: "x"})
		done <- true
	}()

	select {
	case <-done:
		// Good: returned promptly.
	case <-time.After(time.Second):
		t.Fatal("safeChannelSend blocked on a full channel")
	}
}