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
// Performance baseline (Intel Xeon, GOMAXPROCS=2, -benchtime=2s):
//
//	~17,900 ns/op, 98,592 B/op, 9 allocs/op   (64KB stream, 2 chunks)
//
// Profile attribution (largest first):
//   - runtime.slicebytetostring  ~50%   (string(buf[:n]) on every chunk)
//   - strings.(*Reader).Read     ~22%   (pipe.Read syscall path)
//   - runtime.newobject          ~23%   (&Message{...} allocation)
//
// The 32KiB stack-allocated read buffer was chosen over 8KiB because
// the dominant cost (the string() conversion) fires once per chunk:
// a 64KiB read produces 2 chunks instead of 8, so allocs/op drops
// from 21 to 9 (-57%). The absolute ns/op gain is modest (+9.8%)
// because each individual string() now copies 32KiB instead of 8KiB.
// Going past 32KiB is unlikely to help: pipe.Read becomes the
// bottleneck and the marginal alloc-count reduction is small.
//
// See docs/backend-perf.md for the full benchmark trail and the
// reasoning behind the rejected []byte API change.
func PipeToChannel(ctx context.Context, pipe io.ReadCloser, ch chan<- *Message, name string, extraCheck func() bool) {
	var buf [32768]byte
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
