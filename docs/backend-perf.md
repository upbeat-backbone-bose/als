# Backend Performance Baseline

This document records the measured performance of the als backend at a
specific commit, so future changes can be compared against it. The
numbers below come from `go test -bench=. -benchmem -benchtime=2s
-count=1 ./als/client/...` on the listed commit.

## Benchmark results (Intel Xeon, GOMAXPROCS=2)

```
BenchmarkSafeChannelSend-2               839457625   2.847 ns/op   0 B/op   0 allocs/op
BenchmarkSafeChannelSendFullChannel-2    837798340   2.828 ns/op   0 B/op   0 allocs/op
BenchmarkSafeChannelSendCancelled-2      886777369   2.825 ns/op   0 B/op   0 allocs/op
BenchmarkBroadCastMessage-2               18186045 128.6  ns/op  48 B/op   2 allocs/op
BenchmarkPipeToChannel-2                     141825 19834   ns/op 74208 B/op  21 allocs/op
```

Captured at commit `dec60db` (post-audit-fixes, pre-buffer-32K
optimization). The subsequent `52bb0f2` commit increased the
PipeToChannel buffer to 32KiB; see "PipeToChannel optimisation
trail" below.

## Throughput ceilings

| Path | Single-thread | Notes |
|------|---------------|-------|
| Single SSE message send | 350M/sec | Three SafeChannelSend variants are all within 1% of each other -- `select{default}` is one of the cheapest operations in the runtime. |
| Broadcast to N clients | 1 / (128 + 30N) us | 2 allocs per broadcast come from `&Message{...}` and `SnapshotClients()`. Linear in N; for 10 clients = 428ns/call, 100 clients = 3.1us/call. |
| 64KB stream pipe | 50K/sec | Throughput is dominated by `string(buf[:n])` conversion -- see below. |

## PipeToChannel optimisation trail

PipeToChannel is the most expensive path because it processes raw
bytes from a child-process pipe and converts them to strings for SSE
delivery. The 89% of heap allocations came from a single
expression: `&Message{Name: name, Content: string(buf[:n])}`.

### Why 1K -> 8K (commit `9c23467`)

The original implementation used a 1K buffer. At 1K, a 64K read
generates 64 `string()` conversions and 64 `&Message{...}`
allocations per stream. Bumping to 8K dropped that 8x (8 messages
per 64K read).

### Why 8K -> 32K (commit `52bb0f2`)

Profile of the 8K version showed:

| Function | cum % of CPU |
|----------|--------------|
| `runtime.slicebytetostring` | 27.8% |
| `runtime.mallocgc` | 26.5% |
| `runtime.sweepone` (GC) | 21.4% |
| `runtime.memmove` | 10.0% |
| Business logic (PipeToChannel) | 0.3% |

The Message+string expression was 89% of allocations. The
hypothesis: 4x bigger buffer -> 4x fewer string() calls -> 4x
less time. We measured:

| Metric | 8K | 32K | Delta |
|--------|----|----|-------|
| ns/op | 19,834 | 17,900 | **-9.8%** |
| B/op | 74,208 | 98,592 | +32.8% |
| allocs/op | 21 | 9 | **-57.1%** |

**The ns/op gain was modest because the bigger string() calls
take proportionally more time.** Each conversion is now a 32K
memcpy instead of 8K; fewer calls but each more expensive.
Profile of the 32K version confirmed `slicebytetostring` is now
49.6% of CPU (up from 27.8%) but absolute time dropped from
2.43s to 1.76s.

**The real win is allocs/op.** GC pressure is driven by
allocation count, not byte count: 9 allocs/op vs 21 means GC
triggers roughly half as often during sustained iperf3 / speedtest
streams.

### Why we did NOT switch Content to []byte

Tempting next step: change `Message.Content` from `string` to
`[]byte` to eliminate `slicebytetostring` entirely. Rejected
because:

1. Every SSE consumer (`c.SSEvent(name, content)`) requires a
   string at the gin boundary. The `string()` conversion would
   just shift from PipeToChannel to the consumer.
2. The API change ripples through `Message` definition, all
   producer tests, all consumer code, all SSE handlers.
3. Net theoretical improvement: ~30% (eliminating the 50% of CPU
   spent in string() conversion). Realistic improvement after
   shifting cost: 0%.

### Future optimisations, ranked

1. `sync.Pool` of `*Message` (return-to-pool contract needed on
   every consumer) - high effort, ~50% B/op reduction, ~10% ns/op
   gain.
2. `bytes.Buffer` pool for the read buffer - low effort, marginal
   gain (buffer is already stack-allocated).
3. Pre-allocate the `&Message` payload as a fixed-size struct with
   `unsafe` pointer to the buffer - high risk, no public API
   change, ~30% ns/op gain.

None of these are worth doing without sustained throughput issues
from a real workload. Re-measure with the same `go test -bench=.
-benchmem -benchtime=2s` invocation before any further changes.

## How to reproduce these numbers

```sh
cd backend
go test -bench=. -benchmem -benchtime=2s -count=1 -run=^$ ./als/client/...
```

Expected noise: ns/op can vary by ±15% across runs and machines;
B/op is more stable (±5%); allocs/op should be exact on the same
Go version.

If a change makes any of these numbers worse by more than 20%,
profile before merging:

```sh
go test -bench=BenchmarkPipeToChannel -benchtime=5s -cpuprofile=cpu.out -run=^$ ./als/client/...
go tool pprof -top -cum cpu.out
```

Look at `runtime.slicebytetostring`, `runtime.mallocgc`, and
`runtime.memmove` first -- those are the historical hotspots.
