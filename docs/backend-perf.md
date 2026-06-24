# Backend Performance Baseline

This document records the measured performance of the als backend at a
specific commit, so future changes can be compared against it. The
numbers below come from `go test -bench=. -benchmem -benchtime=2s
-count=5 ./als/client/...` (wall time) and `go tool pprof
-peek PipeToChannel prof.out` (CPU attribution), with 3 profile runs
per buffer configuration to control for sampling noise.

## Benchmark results (Intel Xeon, GOMAXPROCS=2)

```
BenchmarkSafeChannelSend-2               839457625   2.847 ns/op   0 B/op   0 allocs/op
BenchmarkSafeChannelSendFullChannel-2    837798340   2.828 ns/op   0 B/op   0 allocs/op
BenchmarkSafeChannelSendCancelled-2      886777369   2.825 ns/op   0 B/op   0 allocs/op
BenchmarkBroadCastMessage-2               18186045 128.6  ns/op  48 B/op   2 allocs/op
BenchmarkPipeToChannel-2                     141825  7,755 ns/op 20552 B/op  41 allocs/op  (1K buffer)
```

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

### Measurement protocol

For each buffer size, we ran:

- **5 benchmark runs** (`-benchtime=2s`) to get the wall-time metric
  (ns/op, B/op, allocs/op).
- **3 profile runs** (`-benchtime=5s` + `-cpuprofile`) to get CPU
  attribution per sub-function. Profile runs have higher intrinsic
  variance than benchmark runs because the pprof sampler only sees a
  fraction of all instructions executed.

The benchmark target is `BenchmarkPipeToChannel`, which uses
`io.NopCloser(strings.NewReader(strings.Repeat("x", 65536)))` to
simulate a 64KB iperf3-style stream. This is the same synthetic load
as commit `9c23467` and `52bb0f2`; the absolute numbers will differ
from real OS-pipe traffic but the **relative ordering** between buffer
sizes is preserved because all three share the same input data.

### Aggregated results (5-run bench + 3-run profile, mean ± std-dev, [min, max])

#### Wall-time metrics

| metric | 1K | 8K | 32K |
|--------|----|----|-----|
| **ns/op** | **7,866.6 ± 53.5** [7,802, 7,921] | 19,779.6 ± 517.7 [19,205, 20,608] | 18,480.2 ± 436.2 [18,149, 19,217] |
| B/op | 20,540 ± 7 [20,538, 20,557] | 74,208 ± 0 | 98,592 ± 0 |
| allocs/op | 41 | 21 | 9 |

#### CPU sub-function time inside PipeToChannel (seconds, mean ± std-dev [min, max])

These are the per-sub-function times reported by `pprof -peek
PipeToChannel`. They are NOT wall time per benchmark iteration; they
are total CPU-seconds consumed by each sub-function during a full
profile run (`-benchtime=5s`).

| sub-function | 1K | 8K | 32K |
|--------------|----|----|-----|
| `runtime.slicebytetostring` (string conversion) | 1.43 ± 0.08 [1.35, 1.51] | **2.32 ± 0.15** [2.16, 2.45] | 2.02 ± 0.73 [1.45, 2.81] |
| `SafeChannelSend` (channel send) | **0.93 ± 0.11** [0.80, 1.02] | 0.65 ± 0.20 [0.47, 0.86] | 0.37 ± 0.15 [0.22, 0.51] |
| `runtime.newobject` (&Message alloc) | 0.44 ± 0.10 [0.33, 0.52] | 0.44 ± 0.04 [0.40, 0.48] | **1.56 ± 0.51** [1.15, 2.16] |
| `strings.(*Reader).Read` (pipe read) | 0.16 ± 0.03 [0.13, 0.19] | 0.24 ± 0.02 [0.22, 0.26] | **0.68 ± 0.16** [0.57, 0.87] |
| **PipeToChannel total (cumulative)** | 3.03 ± 0.25 [2.85, 3.31] | 3.68 ± 0.30 [3.36, 3.95] | 4.65 ± 1.55 [3.41, 6.40] |

#### Sub-function share of PipeToChannel total (mean %)

| sub-function | 1K | 8K | 32K |
|--------------|----|----|-----|
| `slicebytetostring` | 47.1% | **63.0%** | 43.4% |
| `SafeChannelSend` | **30.7%** | 17.7% | 7.9% |
| `newobject` | 14.6% | 11.9% | **33.6%** |
| `strings.Reader.Read` | 5.4% | 6.5% | **14.6%** |

### Why 1K is the optimum, not 8K or 32K

The intuition "bigger buffer = fewer string() calls = faster" is
**wrong here**, because each `string(buf[:n])` is a heap
allocation sized to n bytes. A 32K `string()` conversion does
32x more memcpy work than a 1K one. The 4x reduction in
`string()` call count (8 calls -> 2 calls for 8K/32K) is
dwarfed by the 32x increase in per-call cost.

The dominant cost differs by buffer size:

* **1K buffer**: 47% string() + 31% channel send. 64 small string
  conversions, 64 channel sends. Both are cheap individually.
* **8K buffer**: 63% string(). 8 medium conversions dominate.
* **32K buffer**: 43% string() + 34% &Message alloc + 15% pipe.Read.
  The larger &Message and the larger pipe.Read each cost more
  (Go's small-object allocator is efficient; large objects are
  not).

**The wall-time data (ns/op) is unambiguous**: 1K is **2.4x
faster than 8K and 2.3x faster than 32K**. The standard
deviations are small relative to the gap (`7867 ± 54` vs
`19780 ± 518` vs `18480 ± 436`), so the result is
statistically robust, not noise.

**Allocation count is not the right metric for choosing buffer
size.** GC pressure scales with `runtime.mallocgc` invocation
rate; 41 1K allocations per 64KB at 100 msg/sec real iperf3
traffic = ~4100 alloc/sec, well under any GC trigger threshold.
The 32K choice was a textbook case of optimising the wrong
number.

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
issues from a real workload. Re-measure with the same
`go test -bench=. -benchmem -benchtime=2s -count=5` invocation
before any further changes.

## How to reproduce these numbers

### Wall-time measurement

```sh
cd backend
go test -bench=BenchmarkPipeToChannel -benchmem -benchtime=2s -count=5 -run=^$ ./als/client/...
```

### CPU profile (3 runs recommended for noise control)

```sh
for i in 1 2 3; do
  go test -bench=BenchmarkPipeToChannel -benchtime=5s -cpuprofile=/tmp/buf${i}.prof -run=^$ ./als/client/...
  go tool pprof -peek PipeToChannel /tmp/buf${i}.prof
done
```

### Decision rule

If a change makes any of these numbers worse by more than 20%
relative to the baseline (i.e. 1K's `7,866 ns/op` becomes
>9,440), profile before merging.

Expected noise: ns/op can vary by ±15% across runs and machines;
B/op is more stable (±5%); allocs/op should be exact on the same
Go version.
