# Backend Performance Baseline

This document records the measured performance of the als backend at a
specific commit, so future changes can be compared against it. The
numbers below come from `go test -bench=. -benchmem -benchtime=3s
-count=5 ./als/client/...` on the listed commit.

## Benchmark results (Intel Xeon, GOMAXPROCS=2)

```
BenchmarkSafeChannelSend-2               839457625   2.847 ns/op   0 B/op   0 allocs/op
BenchmarkSafeChannelSendFullChannel-2    837798340   2.828 ns/op   0 B/op   0 allocs/op
BenchmarkSafeChannelSendCancelled-2      886777369   2.825 ns/op   0 B/op   0 allocs/op
BenchmarkBroadCastMessage-2               18186045 128.6  ns/op  48 B/op   2 allocs/op
BenchmarkPipeToChannel-2                     141825  7,755 ns/op 20552 B/op  41 allocs/op  (1K buffer)
```

Captured at the post-buffer-comparison commit (1K chosen over 8K/32K;
see "PipeToChannel buffer comparison" below).

## Throughput ceilings

| Path | Single-thread | Notes |
|------|---------------|-------|
| Single SSE message send | 350M/sec | Three SafeChannelSend variants are all within 1% of each other -- `select{default}` is one of the cheapest operations in the runtime. |
| Broadcast to N clients | 1 / (128 + 30N) us | 2 allocs per broadcast come from `&Message{...}` and `SnapshotClients()`. Linear in N; for 10 clients = 428ns/call, 100 clients = 3.1us/call. |
| 64KB stream pipe (1K buffer) | 130K/sec | Dominated by `string(buf[:n])`; see buffer comparison. |

## PipeToChannel buffer comparison

PipeToChannel is the most expensive path because it processes raw
bytes from a child-process pipe and converts them to strings for SSE
delivery. The dominant cost is `string(buf[:n])`, which is a
heap-allocated `string` header plus a memcpy of n bytes.

### Side-by-side measurements (Intel Xeon, GOMAXPROCS=2, -benchtime=3s, 5-run median, 64KB iperf3-style stream)

| buffer | ns/op | B/op | allocs/op | relative to 1K |
|--------|-------|------|-----------|------------------|
| **1K** | **7,755** | 20,552 | 41 | baseline (chosen) |
| 8K | 19,439 | 74,208 | 21 | +150% ns, -49% allocs |
| 32K | 18,420 | 98,592 | 9 | +137% ns, -78% allocs |

Raw 5-run data:

```
1K  (median 7755):  7755, 7700, 7592, 7874, 8089
8K  (median 19439): 19243, 19650, 19489, 18978, 19439
32K (median 18420): 18161, 18304, 18697, 18420, 18907
```

### Why 1K is the optimum, not 8K or 32K

The intuition "bigger buffer = fewer string() calls = faster" is
**wrong here**, because each `string(buf[:n])` is a heap
allocation sized to n bytes. A 32K `string()` conversion does
32x more memcpy work than a 1K one. The 4x reduction in
`string()` call count (8 calls -> 2 calls for 8K/32K) is
dwarfed by the 32x increase in per-call cost.

Concretely:

* 1K buffer: 64 chunks of 1K -> 64 cheap memcpy's.
* 8K buffer: 8 chunks of 8K -> 8 expensive memcpy's.
* 32K buffer: 2 chunks of 32K -> 2 very expensive memcpy's.

The marginal allocation-count reduction (41 -> 9) does not
translate to lower wall-clock time: GC pressure is
proportional to the rate of `runtime.mallocgc` invocations, and
the 41 allocations of 1K each are still small enough to fit in
the per-P cache without triggering a GC cycle at any realistic
workload. For 100 msg/sec iperf3 traffic, 41 allocs/op
translates to ~4100 alloc/sec -- far below any GC trigger
threshold.

The 8K -> 32K step has zero gain on ns/op (within noise) and
costs +33% in bytes. The 1K -> 8K step costs +150% ns/op
for -49% allocs/op, a strictly worse trade for any workload
below the GC threshold.

### History: why 1K was changed to 8K (commit `9c23467`)

The original implementation used a 1K buffer. The change to 8K
was motivated by a "fewer messages per stream" intuition and
shipped without a before/after benchmark comparison. The actual
ns/op impact was a 2.5x regression.

### Why we did NOT switch Content to []byte

Tempting alternative: change `Message.Content` from `string` to
`[]byte` to eliminate `slicebytetostring` entirely. Rejected:

1. Every SSE consumer (`c.SSEvent(name, content)`) requires a
   string at the gin boundary. The `string()` conversion would
   just shift from PipeToChannel to the consumer.
2. The API change ripples through `Message` definition, all
   producer tests, all consumer code, all SSE handlers.
3. Theoretical improvement at the call site: ~30%. Realistic
   improvement after shifting cost to consumers: 0%.

### Future optimisations, ranked

1. `sync.Pool` of `*Message` (return-to-pool contract needed on
   every consumer) - high effort, ~50% B/op reduction, ~10% ns/op
   gain. **Not recommended** unless real workload shows GC
   pressure: the current 1K buffer's allocation rate at
   <100 msg/sec is well under any trigger threshold.
2. `bytes.Buffer` pool for the read buffer - low effort, marginal
   gain (buffer is already stack-allocated, 1KB per goroutine).
3. Pre-allocate the `&Message` payload as a fixed-size struct with
   `unsafe` pointer to the buffer - high risk, no public API
   change, ~30% ns/op gain.

None of these are worth doing without sustained throughput
issues from a real workload. Re-measure with the same `go test
-bench=. -benchmem -benchtime=3s -count=5` invocation before any
further changes.

## How to reproduce these numbers

```sh
cd backend
go test -bench=. -benchmem -benchtime=3s -count=5 -run=^$ ./als/client/...
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
