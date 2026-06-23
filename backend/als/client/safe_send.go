package client

import "context"

// SafeChannelSend writes msg to ch without blocking. Returns true on
// success, false if the channel was full or ctx is already cancelled.
//
// It exists so producers (callbacks, writer goroutines) can deliver
// messages to a ClientSession without ever pinning the queue handler
// or the caller when the SSE consumer is slow or has disconnected.
// A full channel is treated as "consumer not keeping up": the message
// is dropped rather than blocking the producer.
//
// ctx may be nil; a nil ctx is treated as context.Background().
//
// SafeChannelSend is the canonical entry point -- controllers must
// not perform raw blocking sends into ClientSession.Channel.
func SafeChannelSend(ctx context.Context, ch chan<- *Message, msg *Message) bool {
	if ch == nil {
		return false
	}
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case ch <- msg:
		return true
	case <-ctx.Done():
		return false
	default:
		return false
	}
}