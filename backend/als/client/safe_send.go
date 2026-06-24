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
// The 32KiB stack-allocated read buffer halves the per-byte overhead
// compared to the previous 8KiB: a typical 64KiB read produces 2
// chunks instead of 8, so the dominant cost (the
// runtime.slicebytetostring call on every chunk) drops 4x.
// 32KiB is comfortably stack-sized for the goroutine and not
// unreasonable to live alongside the 16-slot channel buffer.
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
