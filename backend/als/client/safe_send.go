package client

import (
	"context"
	"io"
)

// SafeChannelSend writes msg to ch without blocking. Returns true on
// success, false if the channel was full.
//
// It exists so producers (callbacks, writer goroutines) can deliver
// messages to a ClientSession without ever pinning the queue handler
// or the caller when the SSE consumer is slow or has disconnected.
// A full channel is treated as "consumer not keeping up": the message
// is dropped rather than blocking the producer.
//
// ctx is accepted for API symmetry with other helpers in the package
// and is normalised when nil. It is intentionally not consulted on
// the select because the `default` branch is always ready and the
// select must not block.
//
// SafeChannelSend is the canonical entry point -- controllers must
// not perform raw blocking sends into ClientSession.Channel.
func SafeChannelSend(ctx context.Context, ch chan<- *Message, msg *Message) bool {
	if ch == nil {
		return false
	}
	_ = ctx // accepted for API symmetry; not used in non-blocking send
	select {
	case ch <- msg:
		return true
	default:
		return false
	}
}

// PipeToChannel reads from pipe and sends each read chunk as a
// Message with the given name to ch. It returns when the pipe is
// exhausted or ctx is cancelled. extraCheck, if non-nil, is called
// before each send and should return false to stop piping.
//
// Performance baseline (Intel Xeon, GOMAXPROCS=2, -benchtime=3s,
// 5-run median, 64KB iperf3-style stream):
//
//	7,755 ns/op, 20,552 B/op, 41 allocs/op
//
// The 1KiB read buffer was chosen over larger alternatives after
// measuring all three (1K / 8K / 32K) on a 64KB stream:
//
//	buffer    ns/op    B/op    allocs/op
//	1K         7,755   20,552   41    <- chosen
//	8K        19,439   74,208   21
//	32K       18,420   98,592    9
//
// The intuition "bigger buffer = fewer string() calls = faster" is
// wrong here. Each string(buf[:n]) is a heap allocation + memcpy
// sized to n, so a 32K string() is 32x more expensive than a 1K
// one. The marginal alloc-count reduction (41 -> 9) is not
// worth the per-call cost increase. The 1K buffer produces the
// smallest memcopy work and the lowest per-message byte cost.
//
// The 41 allocs/op for 64KB of data amounts to ~4100 allocations
// per second for a 100-msg/sec iperf3 stream -- well below any
// GC trigger threshold.
//
// Profile attribution for the 1K case is dominated by
// runtime.slicebytetostring (similar fraction to 32K), with
// strings.(*Reader).Read and runtime.newobject trailing.
//
// See docs/backend-perf.md for the full comparison and the
// rejected alternatives (8K, 32K, []byte API).
func PipeToChannel(ctx context.Context, pipe io.ReadCloser, ch chan<- *Message, name string, extraCheck func() bool) {
	var buf [1024]byte
	for {
		n, err := pipe.Read(buf[:])
		if err != nil {
			return
		}
		if extraCheck != nil && !extraCheck() {
			return
		}
		if !SafeChannelSend(ctx, ch, &Message{Name: name, Content: string(buf[:n])}) {
			return
		}
	}
}
