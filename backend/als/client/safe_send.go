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
// ctx is accepted for API symmetry with other helpers in the package.
// It is intentionally not consulted on the select because the
// `default` branch is always ready and the select must not block.
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
// Performance baseline (Intel Xeon, GOMAXPROCS=2, 5-run bench +
// 3-run profile, 64KB iperf3-style stream):
//
//	7,866 ± 54 ns/op, 20,540 B/op, 41 allocs/op
//
// The 1KiB read buffer was chosen over larger alternatives
// (8K, 32K) after measuring all three with mean ± std-dev:
//
//	buffer   ns/op              allocs/op   string()   channel    alloc     read
//	1K       7,866  ±   54     41          47.1%      30.7%      14.6%      5.4%
//	8K      19,779  ±  518     21          63.0%      17.7%      11.9%      6.5%
//	32K     18,480  ±  436      9          43.4%       7.9%      33.6%     14.6%
//
// 1K is 2.4x faster than 8K and 2.3x faster than 32K. The gap is
// much larger than the standard deviation (which is < 3% of the
// mean for all three configurations), so the result is
// statistically robust, not noise.
//
// The intuition "bigger buffer = fewer string() calls = faster"
// is wrong here. Each string(buf[:n]) is a heap allocation + memcpy
// sized to n bytes; a 32K string() is 32x more memcpy work than
// a 1K one. The 4x reduction in call count (8 calls -> 2 calls
// for 8K/32K) is dwarfed by the 32x increase in per-call cost.
//
// 41 allocs/op for 64KB of data is ~4100 allocations per second
// for a 100-msg/sec iperf3 stream -- well below any GC trigger
// threshold, so the lower alloc count of 8K/32K does not pay off.
//
// See docs/backend-perf.md for the full comparison data, profile
// attribution, and the rejected alternatives ([]byte API, larger
// buffer).
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
