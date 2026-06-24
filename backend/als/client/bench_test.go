package client

import (
	"context"
	"io"
	"strings"
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

	go func() {
		for range c1.Channel {
		}
	}()
	go func() {
		for range c2.Channel {
		}
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BroadCastMessage("evt", "data")
	}
}

func BenchmarkPipeToChannel(b *testing.B) {
	ch := make(chan *Message, 1024)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	data := strings.Repeat("x", 65536)
	// 64K data; with 8K buffer → 8 messages per iter
	const msgsPerIter = 8

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := io.NopCloser(strings.NewReader(data))
		go func() { PipeToChannel(ctx, r, ch, "test", nil) }()

		for j := 0; j < msgsPerIter; j++ {
			<-ch
		}
	}
}
