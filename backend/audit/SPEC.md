# SPEC: ALS Backend Audit & Hardening (Tier 2)

**Branch:** `codex-260810-project-audit`
**Base:** `master` @ `3133f90f`
**Skill:** old-coder (Tier 2 — bug fix / small feature)
**Spec approval:** autonomous run (no human pre-approval; correlation-breaking review not performed)

## 0. Baseline (the "test confirm" step — already executed)

- `go test -short ./...` (16 packages) — **all PASS** under `-short` and under
  plain (no `-race`) coverage runs. Five consecutive full-suite runs all PASS.
- Coverage: **70.0%** total (CI threshold 52%; safe headroom of 18 pts).
- One baseline failure observed *once* on the very first `go test -race -coverprofile=…
  -covermode=atomic ./…` run: `TestHandleSpeedtestDotNetSuccess` reported
  `status=500; body={"error":"exit status 1","success":false}` after a 5.61s run.
  Re-running the speedtest package in isolation (5x), without `-race`, with
  `-coverprofile` (3x), and in the full suite without `-race` (5x): all PASS.
  Conclusion: the test's own 5 s request deadline fires under heavy CI load
  because the goroutine kill on cancel produces `cmd.Wait → "exit status 1"`.
  This is a flaky test, not a production bug, but it still breaks CI.
- `gosec ./...` reported 3 issues (saved to `audit/d2-gosec.json`):
  1. **G112 — http.Server has no ReadHeaderTimeout** (`http/init.go:61-64`).
     Slowloris attack surface on every deployment. **Real, in-scope.**
  2. **G118 — cancel not called** (`als/client/client.go:43`). The cancel
     function is invoked from the watching goroutine — only a leak if both
     parent and request contexts never fire. In production they always do.
     **Document with regression test; do not change the API.**
  3. **G118 — cancel not called** (`als/controller/session/session.go:69`).
     False positive — `cancel()` is already deferred on line 80.
- `govulncheck` could not reach `vuln.go.dev` (sandbox network policy blocks
  external HTTP). Recorded as skipped layer in EVIDENCE.
- Pre-existing `checkOrigin` security TODO in `als/controller/shell/shell.go`
  is **out of scope** for this audit: an explicit, table-driven test already
  documents current behavior with `TODO(security)` tags, and the fix is a
  design conversation, not a bug fix. Noting it as a known limit.

## 1. Behaviors (acceptance criteria)

Each behavior is the unit a regression test verifies. The mapping table at the
end lists test → behavior.

### B1. HTTP server timeouts prevent Slowloris

`http.Server` constructed by `http.Server.Start()` MUST set non-zero values
for `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, and `IdleTimeout`,
so that a peer that opens a connection and never sends a complete header
cannot keep it open indefinitely.

- A regression test inspects the `http.Server` returned by the package after
  `Start()` returns the configured server (or after a new constructor that
  exposes the configured server for testing) and asserts each field is > 0.
- The test is allowed to reach into package internals via a `var newServer
  = func(...) *http.Server { return &http.Server{…} }` indirection so the
  assertion does not need to bind a port.

### B2. Speedtest handler test is deterministic under CI load

`TestHandleSpeedtestDotNetSuccess` MUST pass deterministically when the
speedtest package is run as part of the full suite under
`go test -race -coverprofile=… -covermode=atomic ./…`, on this Windows host
running 5 consecutive full-suite runs.

- Concretely: the test's request context deadline MUST be long enough that
  `cmd.Wait()` returns normally (`exit 0`) before the deadline fires, even
  when the host is busy running 16 packages in parallel.
- A separate **flakiness guard test** MAY be added (table-driven, parameterised
  on deadline) to lock the new deadline in. The CI run in EVIDENCE then
  becomes the authoritative check.

### B3. `ClientSession.GetContext` does not leak the watcher goroutine

When the *parent* context (`c.ctx`, the one stored by `SetContext`) is
cancelled, `GetContext`'s watcher goroutine MUST terminate within a short
window (≤ 1 s), without depending on the caller's request context.

- A regression test:
  1. Creates a `ClientSession`.
  2. Calls `SetContext(parentCtx)` where `parentCtx` is a child of a
     caller-controlled `WithCancel` parent.
  3. Calls `GetContext(context.Background())` to obtain a derived `ctx`.
  4. Cancels `parentCtx`.
  5. Asserts `ctx.Done()` fires within 1 s (proves the goroutine noticed and
     called `cancel`).
  6. Repeats the same scenario but cancels the request context (the one
     passed to `GetContext`) — `ctx.Done()` must also fire within 1 s.
- The test does not assert goroutine count (Go test harness makes that
  brittle) but asserts the *observable* contract: the returned context
  is cancelled.

## 2. Invariants (must NOT change)

Each negative clause must hold after this change, with zero NEW failures in
the listed layers. Existing baseline failures: none (full suite PASS).

- All 16 backend packages continue to compile, `go vet`, and test PASS.
- `TestHandleSpeedtestDotNetSuccess` does NOT lose its 200-status assertion
  (i.e. it must still call `speedtest`, not just return 200 unconditionally).
- No change to the public `ClientSession` method signatures.
- No change to the JSON shape returned by `Handle` (the route signatures
  are public; the UI consumes them).
- `golangci-lint` rules in `backend/.golangci.yml` remain satisfied — no
  new `//nolint:` lines added to silence the lint instead of fixing the
  underlying code.
- Total coverage does NOT drop below 70.0% (current baseline).
- No new third-party dependencies. `audit/gocache`, `audit/gomodcache`,
  `audit/gotmpdir` are workspace-local cache dirs, not deps.
- The `checkOrigin` policy in `shell.go` is **explicitly unchanged** (out of
  scope; test still asserts current behavior).

## 3. Setup plan (environment changes authorised by this SPEC)

The sandbox blocks:
- Writing under `~/.git/` (deny ACEs). Branch creation required escalated
  permissions — already done, branch `codex-260810-project-audit` exists.
- Network egress to `proxy.golang.org`, `vuln.go.dev`, `registry.npmjs.org`.
  All `go build/test` runs in this session use:
  - `GOPROXY=off`
  - `GOMODCACHE=C:\Users\RayDell\go\pkg\mod` (pre-populated)
  - `GOCACHE=E:\codeBase\als\backend\audit\gocache` (workspace-local)
  - `CGO_ENABLED=0` (race detector disabled — cgo not available)
- `go test -race` is therefore unavailable; the gauntlet runs without it
  and notes the limitation in EVIDENCE.

No new dependencies will be installed. No new tooling. The local cache
directories under `backend/audit/` (gocache, gomodcache, gotmpdir) are
created from this SPEC.

## 4. Files the gauntlet will add or modify

- `backend/http/init.go` — add timeout fields to `http.Server{}` in
  `(*Server).Start`.
- `backend/http/init_test.go` — new test for B1 (or extend existing file).
- `backend/als/controller/speedtest/speedtest_cli_test.go` — extend the
  request deadline in `TestHandleSpeedtestDotNetSuccess` for B2 (or extract
  the constant and assert it).
- `backend/als/client/client_test.go` — add B3 goroutine-termination test.
- `backend/audit/SPEC.md` (this file) and `backend/audit/EVIDENCE.md`
  (final report).
- `backend/audit/*.log` — per-layer command output (already accumulating).

No other files are touched. No UI changes.

## 5. Tier justification

Tier 2 (bug fix / small feature). Domain: a public-facing network diagnostic
server — so security hardening (B1) is a Tier-3-shaped risk; for THIS audit
the fix is a single 4-line timeout block, which is the smallest unit of
change, so it stays in Tier 2 with the security framing documented. Tier-3
adversarial pass (hostile input fuzzing) is not added because the
attack surface change is a single timeout configuration; EVIDENCE will note
this as a known limit. If a future audit tackles `checkOrigin`, that one
will be Tier 3 with property-based tests on the WebSocket handshake.

## 6. Mapping (behaviour → test)

| Behaviour | Test file | Test function |
|---|---|---|
| B1 (Slowloris timeouts) | `backend/http/init_test.go` (new or extended) | `TestServerTimeoutsAreSet` |
| B2 (deterministic speedtest) | `backend/als/controller/speedtest/speedtest_cli_test.go` | deadline bump + 5x CI loop in EVIDENCE |
| B3 (GetContext goroutine terminates) | `backend/als/client/client_test.go` | `TestGetContextTerminatesOnParentCancel`, `TestGetContextTerminatesOnRequestCancel` |
