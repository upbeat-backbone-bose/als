package timer

import (
	"context"
	"testing"
	"time"

	"github.com/samlm0/als/v2/als/client"
)

func TestUpdateSystemResourceBroadcastsMemoryUsage(t *testing.T) {
	clearTestClients(t)

	ch := make(chan time.Time, 4)
	prev := tickerFactoryForTest
	tickerFactoryForTest = func() <-chan time.Time { return ch }
	t.Cleanup(func() { tickerFactoryForTest = prev })

	session := &client.ClientSession{
		Channel:   make(chan *client.Message, 4),
		CreatedAt: time.Now(),
	}
	client.AddClient("memory-test", session)
	t.Cleanup(func() { client.RemoveClient("memory-test") })

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		UpdateSystemResourceContext(ctx)
	}()

	// Drive two ticks.
	ch <- time.Now()
	ch <- time.Now()

	// Expect two MemoryUsage messages on the channel.
	got := 0
	deadline := time.Now().Add(time.Second)
	for got < 2 {
		select {
		case msg := <-session.Channel:
			if msg.Name != "MemoryUsage" {
				t.Errorf("message name = %q; want MemoryUsage", msg.Name)
			}
			if msg.Content == "" {
				t.Error("MemoryUsage content is empty")
			}
			got++
		case <-time.After(time.Until(deadline)):
			t.Fatalf("got %d MemoryUsage messages; want 2", got)
		}
	}

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("UpdateSystemResource did not exit after cancel")
	}
}

func TestUpdateSystemResourceExitsOnCancelBeforeTick(t *testing.T) {
	prev := tickerFactoryForTest
	tickerFactoryForTest = func() <-chan time.Time {
		// Channel that never produces a tick.
		return make(chan time.Time)
	}
	t.Cleanup(func() { tickerFactoryForTest = prev })

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		UpdateSystemResourceContext(ctx)
	}()
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("UpdateSystemResource did not exit after cancel")
	}
}

// clearTestClients resets the client map for tests that broadcast.
func clearTestClients(t *testing.T) {
	t.Helper()
	client.RemoveAllClients()
	t.Cleanup(client.RemoveAllClients)
}
