package client

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

func resetClientForBench() {
	clientsMu.Lock()
	Clients = make(map[string]*ClientSession)
	clientsMu.Unlock()
}

func BenchmarkSafeChannelSend(b *testing.B) {
	ch := make(chan *Message, 100)
	ctx := context.Background()
	msg := &Message{Name: "evt", Content: "data"}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		SafeChannelSend(ctx, ch, msg)
	}
}

func BenchmarkSafeChannelSendFullChannel(b *testing.B) {
	ch := make(chan *Message, 1)
	ch <- &Message{Name: "filler"}
	ctx := context.Background()
	msg := &Message{Name: "evt", Content: "data"}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		SafeChannelSend(ctx, ch, msg)
	}
}

func BenchmarkSafeChannelSendCancelled(b *testing.B) {
	ch := make(chan *Message, 1)
	ch <- &Message{Name: "filler"}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	msg := &Message{Name: "evt", Content: "data"}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		SafeChannelSend(ctx, ch, msg)
	}
}

func BenchmarkBroadCastMessage(b *testing.B) {
	resetClientForBench()
	c1 := &ClientSession{Channel: make(chan *Message, 100), CreatedAt: time.Now()}
	c2 := &ClientSession{Channel: make(chan *Message, 100), CreatedAt: time.Now()}
	Clients["c1"] = c1
	Clients["c2"] = c2

	c1Done := make(chan struct{})
	c2Done := make(chan struct{})
	go func() {
		defer close(c1Done)
		for range c1.Channel {
		}
	}()
	go func() {
		defer close(c2Done)
		for range c2.Channel {
		}
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BroadCastMessage("evt", "data")
	}
	b.StopTimer()

	close(c1.Channel)
	close(c2.Channel)
	<-c1Done
	<-c2Done
}

func BenchmarkPipeToChannel(b *testing.B) {
	ch := make(chan *Message, 16)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	data := strings.Repeat("x", 65536)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := io.NopCloser(strings.NewReader(data))
		done := make(chan struct{})
		go func() {
			defer close(done)
			PipeToChannel(ctx, r, ch, "test", nil)
		}()

	drain:
		for {
			select {
			case <-ch:
			case <-done:
				break drain
			}
		}
	}
}

// TestPipeToChannelSignalsDoneAfterDrain pins the contract that
// `done` closes only after the goroutine has finished draining the
// pipe and sending every read chunk. This guards against a
// regression where the goroutine leaks and the done channel is
// closed prematurely (before all bytes are read or sent), which
// would silently lose messages in production.
func TestPipeToChannelSignalsDoneAfterDrain(t *testing.T) {
	t.Parallel()

	ch := make(chan *Message, 16)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 16K data with an 8K buffer -> exactly 2 messages must be
	// delivered. Use 'a' so each chunk is distinguishable from
	// other tests that share the channel name.
	data := strings.Repeat("a", 16384)
	pipe := io.NopCloser(strings.NewReader(data))

	done := make(chan struct{})
	go func() {
		defer close(done)
		PipeToChannel(ctx, pipe, ch, "evt", nil)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("PipeToChannel did not signal done within 2s")
	}

	// After done fires, every chunk the goroutine read from the
	// pipe must have been written to ch. Drain the channel and
	// assert the total content.
	var got strings.Builder
drain:
	for {
		select {
		case msg := <-ch:
			got.WriteString(msg.Content)
		default:
			break drain
		}
	}

	if got.String() != data {
		t.Errorf("got %d bytes; want %d", got.Len(), len(data))
	}
}

func BenchmarkWaitQueueCycle(b *testing.B) {
	queueEntries = nil
	queueShutdown = make(chan struct{})
	queueShutdownOnce = sync.Once{}

	handlerCtx, handlerCancel := context.WithCancel(context.Background())
	defer handlerCancel()
	handlerReady := make(chan struct{})
	go func() {
		close(handlerReady)
		HandleQueue(handlerCtx)
	}()
	<-handlerReady

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		callerCtx, callerCancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			defer close(done)
			WaitQueue(callerCtx, func() { callerCancel() })
		}()
		<-done
	}
}

// BenchmarkGracefulShutdown measures how long it takes for N parked
// WaitQueue callers to unblock after the handler's context is cancelled.
// Each goroutine signals a barrier after enqueuing so the benchmark can
// synchronise before cancelling.
// BenchmarkShutdownQueue measures the raw cost of ShutdownQueue plus
// queueShutdown channel close. Entries are pre-created once; the
// benchmark resets only the shutdown signal per iteration.
func BenchmarkShutdownQueue(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000}
	for _, n := range sizes {
		entries := make([]*queueEntry, n)
		for j := 0; j < n; j++ {
			_, cancel := context.WithCancel(context.Background())
			entries[j] = &queueEntry{
				ctx:    context.Background(),
				cancel: cancel,
			}
		}

		b.Run(fmt.Sprintf("entries=%d", n), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				queueEntries = append(queueEntries[:0], entries...)
				queueShutdown = make(chan struct{})
				queueShutdownOnce = sync.Once{}
				b.StartTimer()
				ShutdownQueue()
			}
		})
	}
}
